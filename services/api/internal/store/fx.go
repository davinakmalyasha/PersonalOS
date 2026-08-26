package store

import (
	"strings"
)

// ---- Multi-currency FX (phase 13a) ----
//
// Rates are relative to one base currency (app_meta key fx_base, default IDR).
// SEMANTICS: rate_to_base multiplies the STORED MINOR UNIT of the source
// currency into base minor units (e.g. USD cents -> IDR: ~160 when 1 USD =
// 16,000 IDR). A missing rate row means "already base / treat as 1:1", so
// single-currency users pay zero complexity.

type ExchangeRate struct {
	Code       string  `json:"code"`
	RateToBase float64 `json:"rate_to_base"`
	UpdatedAt  string  `json:"updated_at"`
}

func (f *Finance) BaseCurrency() string {
	var base string
	_ = f.DB.QueryRow(`SELECT value FROM app_meta WHERE key='fx_base'`).Scan(&base)
	if strings.TrimSpace(base) == "" {
		return "IDR"
	}
	return base
}

// SetBaseCurrency changes the reporting base; existing rates keep meaning
// ("1 USD = N base"), so callers should refresh them after switching.
func (f *Finance) SetBaseCurrency(code string) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" || len(code) > 8 {
		return ErrInvalid
	}
	_, err := f.DB.Exec(`
		INSERT INTO app_meta (key,value,updated_at) VALUES ('fx_base',?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		code, NowRFC3339())
	return err
}

// UpsertRate stores 1 unit of code = rate × base.
func (f *Finance) UpsertRate(code string, rate float64) (ExchangeRate, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" || len(code) > 8 || rate <= 0 {
		return ExchangeRate{}, ErrInvalid
	}
	now := NowRFC3339()
	_, err := f.DB.Exec(`
		INSERT INTO exchange_rates (code,rate_to_base,updated_at) VALUES (?,?,?)
		ON CONFLICT(code) DO UPDATE SET rate_to_base=excluded.rate_to_base, updated_at=excluded.updated_at`,
		code, rate, now)
	if err != nil {
		return ExchangeRate{}, err
	}
	logChange(f.DB, "fx_rate", code, "update", "rate refreshed")
	var out ExchangeRate
	err = f.DB.QueryRow(`SELECT code,rate_to_base,updated_at FROM exchange_rates WHERE code=?`, code).
		Scan(&out.Code, &out.RateToBase, &out.UpdatedAt)
	return out, err
}

func (f *Finance) ListRates() ([]ExchangeRate, error) {
	rows, err := f.DB.Query(`SELECT code,rate_to_base,updated_at FROM exchange_rates ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExchangeRate{}
	for rows.Next() {
		var r ExchangeRate
		if err := rows.Scan(&r.Code, &r.RateToBase, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// rateLookup loads all rates + the base once per operation.
type rateLookup struct {
	base   string
	rates  map[string]float64
}

func (f *Finance) loadRates() (*rateLookup, error) {
	l := &rateLookup{base: f.BaseCurrency(), rates: map[string]float64{}}
	rows, err := f.DB.Query(`SELECT code,rate_to_base FROM exchange_rates`)
	if err != nil {
		return l, err
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		var rate float64
		if err := rows.Scan(&code, &rate); err != nil {
			return l, err
		}
		l.rates[code] = rate
	}
	return l, rows.Err()
}

// toBase converts a minor-unit amount in `currency` into the base currency,
// rounding half away from zero. Unknown currency → 1:1 passthrough.
func (l *rateLookup) toBase(currency string, amount int64) int64 {
	if currency == "" || currency == l.base {
		return amount
	}
	rate, ok := l.rates[currency]
	if !ok || rate <= 0 {
		return amount
	}
	v := float64(amount) * rate
	if v >= 0 {
		return int64(v + 0.5)
	}
	return int64(v - 0.5)
}
