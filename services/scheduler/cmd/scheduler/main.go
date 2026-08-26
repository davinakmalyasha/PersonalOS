// Personal OS scheduler: a tiny Go loop that runs nightly maintenance passes
// (transfer pairing, subscription recompute, expiring digest, budget-over
// check) against the API over HTTP and nudges via Telegram/Discord when
// anything needs attention. It never touches the DB directly.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/davinakmalyasha/PersonalOS/services/scheduler/internal/config"
	"github.com/davinakmalyasha/PersonalOS/services/scheduler/internal/jobs"
	"github.com/davinakmalyasha/PersonalOS/services/scheduler/internal/notify"
)

func main() {
	cfg := config.Load()
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Str("svc", "scheduler").Logger()

	var channels notify.Multi
	if cfg.TelegramBotToken != "" && cfg.TelegramChatID != "" {
		channels = append(channels, notify.Telegram{BotToken: cfg.TelegramBotToken, ChatID: cfg.TelegramChatID})
	}
	if cfg.DiscordWebhook != "" {
		channels = append(channels, notify.Discord{WebhookURL: cfg.DiscordWebhook})
	}

	runner := &jobs.Runner{Client: jobs.NewClient(cfg.APIURL, cfg.APIToken)}
	runner.LowBalanceDays = cfg.LowBalanceDays
	runner.LowBalanceThreshold = cfg.LowBalanceThreshold
	interval := time.Duration(cfg.IntervalSeconds) * time.Second

	logger.Info().
		Str("api", cfg.APIURL).
		Dur("interval", interval).
		Int("run_hour_utc", cfg.RunHourUTC).
		Int64("low_balance_threshold", cfg.LowBalanceThreshold).
		Bool("notify", len(channels) > 0).
		Msg("scheduler starting")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cfg.RunOnStart {
		pass(ctx, runner, channels, logger, time.Now())
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	lastRunDay := ""
	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("shutdown")
			return
		case now := <-ticker.C:
			if !dueNow(cfg, now, &lastRunDay) {
				continue
			}
			pass(ctx, runner, channels, logger, now)
		}
	}
}

// dueNow implements nightly gating: without SCHED_RUN_HOUR every tick runs;
// with it, exactly one pass fires when the UTC hour matches each day.
func dueNow(cfg config.Config, now time.Time, lastRunDay *string) bool {
	if cfg.RunHourUTC < 0 || cfg.RunHourUTC > 23 {
		return true
	}
	day := now.UTC().Format("2006-01-02")
	if now.UTC().Hour() != cfg.RunHourUTC || day == *lastRunDay {
		return false
	}
	*lastRunDay = day
	return true
}

func pass(ctx context.Context, runner *jobs.Runner, channels notify.Multi, logger zerolog.Logger, now time.Time) {
	digest, errs := runner.Run(ctx, now)
	for _, err := range errs {
		logger.Warn().Err(err).Msg("job failed")
	}
	text := digest.Render(now)
	if text == "" {
		logger.Info().Msg("pass complete — nothing notable")
		return
	}
	if len(channels) == 0 {
		logger.Info().Msgf("findings (no webhook configured):\n%s", text)
		return
	}
	if err := channels.Notify(ctx, text); err != nil {
		logger.Error().Err(err).Msg("notify failed")
		return
	}
	logger.Info().Int("channels", len(channels)).Msg("digest delivered")
}
