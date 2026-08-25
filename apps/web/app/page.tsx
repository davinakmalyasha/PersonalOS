"use client";

// Live bento board. Preview tiles per the approved mock; click → the tile
// grows into a full detail card (stacked-cards feel) while siblings dim and
// recede. Agents drive it: /?expand=<tile>, `personal-os:focus` events, and
// activity-pulse highlight rings on writes.

import { Suspense, useCallback, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { X } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { SummaryCards } from "@/components/finance/summary-cards";
import { CategoryChart } from "@/components/finance/category-chart";
import { BudgetBars } from "@/components/finance/budget-bars";
import { TransactionsTable } from "@/components/finance/transactions-table";
import { TodayColumn } from "@/components/planner/today-column";
import { WeightChart } from "@/components/health/weight-chart";
import { WorkoutBars } from "@/components/health/workout-bars";
import { GroceryChecklist } from "@/components/health/grocery-checklist";
import { QuickAdd } from "@/components/knowledge/quick-add";
import {
  apiGet,
  apiSend,
  currentMonth,
  todayStr,
  type Account,
  type GroceryItem,
  type HealthSummary,
  type KnowledgeItem,
  type Meal,
  type MonthSummary,
  type Occurrence,
  type Task,
  type TodayBundle,
  type WeightPoint,
  type Workout,
} from "@/lib/api";
import { usePillarVersion } from "@/lib/use-activity";

type TileId = "today" | "money" | "body" | "upcoming" | "captures" | "grocery";

const TILES: { id: TileId; pillar: string }[] = [
  { id: "today", pillar: "planner" },
  { id: "money", pillar: "finance" },
  { id: "body", pillar: "health" },
  { id: "upcoming", pillar: "planner" },
  { id: "captures", pillar: "universal" },
  { id: "grocery", pillar: "health" },
];

// ---- generic versioned fetch ----

function useTileData<T>(path: string, pillar: string, transform?: (r: unknown) => T): T | null {
  const version = usePillarVersion(pillar);
  const [data, setData] = useState<T | null>(null);
  useEffect(() => {
    let alive = true;
    apiGet<unknown>(path)
      .then((r) => {
        if (alive) setData(transform ? transform(r) : (r as T));
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path, version]);
  return data;
}

// ---- small preview atoms ----

function Metric({ label, value, sub, strong }: { label: string; value: string; sub?: string; strong?: boolean }) {
  return (
    <div>
      <p className="text-[10px] uppercase tracking-wide text-muted-foreground">{label}</p>
      <p className={`font-mono tabular-nums ${strong ? "text-xl font-semibold" : "text-lg"}`}>{value}</p>
      {sub && <p className="text-[10px] text-muted-foreground">{sub}</p>}
    </div>
  );
}

function Row({ left, right, dim }: { left: string; right: string; dim?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-2 py-0.5 text-xs">
      <span className={`truncate ${dim ? "text-muted-foreground" : ""}`}>{left}</span>
      <span className="shrink-0 font-mono tabular-nums text-muted-foreground">{right}</span>
    </div>
  );
}

// ---- tiles ----

function TodayPreview({ data }: { data: TodayBundle | null }) {
  const overdue = data?.overdue.length ?? 0;
  return (
    <>
      <div className="grid grid-cols-2 gap-3">
        <Metric label="Overdue" value={String(overdue)} strong={overdue > 0} />
        <Metric label="Due today" value={String(data?.due_today.length ?? 0)} />
        <Metric
          label="Habits"
          value={`${(data?.habits ?? []).filter((h) => h.done_today).length}/${(data?.habits ?? []).length}`}
        />
        <Metric label="Events" value={String(data?.events.length ?? 0)} />
      </div>
      <div className="mt-2 space-y-0.5">
        {(data?.due_today ?? []).slice(0, 3).map((t) => (
          <Row key={t.id} left={t.title} right={t.priority} dim />
        ))}
      </div>
    </>
  );
}

function TodayDetail({ data, reload }: { data: TodayBundle | null; reload: () => void }) {
  const toggleTask = async (t: Task) => {
    await apiSend(`/v1/tasks/${t.id}`, "PATCH", { status: t.status === "done" ? "todo" : "done" });
    reload();
  };
  const toggleHabit = async (habitId: string) => {
    await apiSend(`/v1/habits/${habitId}/checkoffs`, "POST", { date: todayStr() });
    reload();
  };
  return (
    <TodayColumn
      overdue={data?.overdue ?? []}
      dueToday={data?.due_today ?? []}
      habits={data?.habits ?? []}
      events={data?.events ?? []}
      onToggleTask={toggleTask}
      onToggleHabit={toggleHabit}
    />
  );
}

function MoneyPreview({ s }: { s: MonthSummary | null }) {
  const top = [...(s?.by_category ?? [])].sort((a, b) => b.spent_minor - a.spent_minor)[0];
  return (
    <div className="grid grid-cols-2 gap-3">
      <Metric label="In" value={`+${((s?.income_minor ?? 0) / 100).toLocaleString("id-ID")}`} />
      <Metric label="Out" value={`−${((s?.outcome_minor ?? 0) / 100).toLocaleString("id-ID")}`} />
      <Metric label="Net" value={`${((s?.net_minor ?? 0) / 100).toLocaleString("id-ID")}`} strong />
      <Metric label="Top cat" value={top?.name ?? "—"} sub={top ? `${(top.spent_minor / 100).toLocaleString("id-ID")}` : undefined} />
      <div className="col-span-2">
        <GoalsStrip />
      </div>
    </div>
  );
}

function GoalsStrip() {
  const goals = useTileData<{ items: { id: string; name: string; kind: string; target_minor: number | null; saved_minor: number }[] }>(
    "/v1/goals?kind=savings",
    "finance",
  );
  const subs = useTileData<{ items: unknown[] }>("/v1/finance/recurring", "finance");
  const items = (goals?.items ?? []).slice(0, 2);
  if (items.length === 0 && (subs?.items ?? []).length === 0) return null;
  return (
    <div className="mt-2 space-y-1 border-t pt-2">
      {items.map((g) => {
        const pct = g.target_minor ? Math.min(100, Math.round((g.saved_minor / g.target_minor) * 100)) : 0;
        return (
          <div key={g.id} className="flex items-center gap-2 text-xs">
            <span className="w-24 truncate">{g.name}</span>
            <div className="h-1 flex-1 overflow-hidden rounded-full bg-muted">
              <div className="h-full rounded-full bg-foreground" style={{ width: `${pct}%` }} />
            </div>
            <span className="font-mono tabular-nums text-muted-foreground">{pct}%</span>
          </div>
        );
      })}
      {(subs?.items ?? []).length > 0 && (
        <p className="text-[10px] text-muted-foreground">
          {(subs?.items ?? []).length} recurring subscription{(subs?.items ?? []).length === 1 ? "" : "s"} detected
        </p>
      )}
    </div>
  );
}

function MoneyDetail({ month }: { month: string }) {
  const money = useTileData<MonthSummary>(`/v1/finance/summary?month=${month}`, "finance");
  const accounts = useTileData<{ items: Account[] }>("/v1/accounts", "finance");
  const accountId = accounts?.items?.[0]?.id ?? "";
  return (
    <div className="space-y-4 overflow-y-auto pr-1" style={{ maxHeight: "calc(100vh - 12rem)" }}>
      <SummaryCards
        income={money?.income_minor ?? 0}
        outcome={money?.outcome_minor ?? 0}
        net={money?.net_minor ?? 0}
      />
      <div className="grid gap-4 lg:grid-cols-5">
        <div className="lg:col-span-3">
          <CategoryChart data={money?.by_category ?? []} />
        </div>
        <div className="lg:col-span-2">
          <BudgetBars budgets={money?.budgets ?? []} />
        </div>
      </div>
      {accountId && <TransactionsTable accountId={accountId} />}
    </div>
  );
}

function BodyPreview({ s }: { s: HealthSummary | null }) {
  const change = s?.weight.change_kg;
  const goal = s?.calorie_goal;
  const eaten = s?.calories_today;
  return (
    <div className="grid grid-cols-2 gap-3">
      <Metric
        label="Weight"
        value={s?.weight.latest_kg != null ? `${s.weight.latest_kg.toFixed(1)}kg` : "—"}
        sub={change != null ? `${change > 0 ? "+" : ""}${change.toFixed(1)} kg` : undefined}
        strong
      />
      <Metric label="Workouts 14d" value={String(s?.workouts.count ?? 0)} sub={`${s?.workouts.total_minutes ?? 0} min`} />
      <Metric
        label="Calories today"
        value={eaten != null ? eaten.toLocaleString("id-ID") : "—"}
        sub={goal != null ? `of ${goal.toLocaleString("id-ID")} goal` : undefined}
      />
      <Metric
        label="Water today"
        value={s?.water_today_ml != null ? `${(s.water_today_ml / 1000).toFixed(2)}L` : "—"}
      />
    </div>
  );
}

// Time-relative fetch paths are inherently impure during render; the values
// are stable per page load and only feed cache keys.
 
function isoDaysAgo(n: number): string {
  return new Date(Date.now() - n * 86400000).toISOString().slice(0, 10);
}

function BodyDetail() {
  const from = isoDaysAgo(89);
  const to = todayStr();
  const weights = useTileData<{ points: WeightPoint[] }>(
    `/v1/health/weight-series?from=${from}&to=${to}`,
    "health",
  );
  const workouts = useTileData<{ items: Workout[] }>(
    `/v1/workouts?from=${isoDaysAgo(13)}&to=${to}&page_size=100`,
    "health",
  );
  const meals = useTileData<{ items: Meal[] }>(
    `/v1/meals?from=${isoDaysAgo(6)}&to=${to}&page_size=50`,
    "health",
  );
  return (
    <div className="space-y-4 overflow-y-auto pr-1" style={{ maxHeight: "calc(100vh - 12rem)" }}>
      <WeightChart points={weights?.points ?? []} />
      <WorkoutBars workouts={workouts?.items ?? []} />
      <RecentMini meals={meals?.items ?? []} workouts={workouts?.items ?? []} />
    </div>
  );
}

function RecentMini({ meals, workouts }: { meals: Meal[]; workouts: Workout[] }) {
  const entries = [
    ...meals.map((m) => ({ at: m.eaten_at, title: m.title, kind: "meal" })),
    ...workouts.map((w) => ({ at: w.performed_at, title: w.title ?? "Training", kind: "workout" })),
  ]
    .sort((a, b) => (a.at < b.at ? 1 : -1))
    .slice(0, 8);
  return (
    <ul className="space-y-1">
      {entries.map((e) => (
        <li key={e.kind + e.at + e.title} className="flex items-center gap-2 rounded-md border p-2 text-xs">
          <span className="font-mono tabular-nums text-muted-foreground">{new Date(e.at).toISOString().slice(5, 16).replace("T", " ")}</span>
          <span className="truncate">{e.title}</span>
          <span className="ml-auto rounded-full border px-1.5 font-mono text-[9px] uppercase text-muted-foreground">{e.kind}</span>
        </li>
      ))}
      {entries.length === 0 && <li className="rounded-md border border-dashed p-3 text-center text-muted-foreground">Nothing logged.</li>}
    </ul>
  );
}

function UpcomingPreview({ data }: { data: { items: { date: string; tasks: Task[]; events: Occurrence[] }[] } | null }) {
  const flat = (data?.items ?? []).flatMap((d) => [
    ...d.events.map((e) => ({ date: d.date, title: e.title, kind: "event" })),
    ...d.tasks.map((t) => ({ date: d.date, title: t.title, kind: "task" })),
  ]).slice(0, 5);
  return (
    <div className="space-y-0.5">
      {flat.map((e, i) => (
        <Row key={i} left={`${e.date.slice(5)} · ${e.title}`} right={e.kind} dim={e.kind === "task"} />
      ))}
      {flat.length === 0 && <p className="text-xs text-muted-foreground">Clear week ahead.</p>}
    </div>
  );
}

function UpcomingDetail({ data }: { data: { items: { date: string; tasks: Task[]; events: Occurrence[] }[] } | null }) {
  return (
    <div className="space-y-3 overflow-y-auto pr-1" style={{ maxHeight: "calc(100vh - 12rem)" }}>
      {(data?.items ?? []).map((d) => (
        <div key={d.date}>
          <h4 className="mb-1 font-mono text-[11px] uppercase tracking-wide text-muted-foreground">{d.date}</h4>
          {d.tasks.length === 0 && d.events.length === 0 ? (
            <p className="rounded-md border border-dashed p-2 text-xs text-muted-foreground">—</p>
          ) : (
            <ul className="space-y-1">
              {d.events.map((e) => (
                <li key={e.event_id + e.date} className="flex items-center gap-2 rounded-md border p-2 text-sm">
                  <span className="font-mono text-xs text-muted-foreground">{new Date(e.starts_at).toISOString().slice(11, 16)}</span>
                  {e.title}
                  {e.series && <span className="ml-auto rounded-full border px-1.5 font-mono text-[9px] text-muted-foreground">recurring</span>}
                </li>
              ))}
              {d.tasks.map((t) => (
                <li key={t.id} className="flex items-center gap-2 rounded-md border border-dashed p-2 text-sm">
                  <span className="font-mono text-xs text-muted-foreground">due</span>
                  {t.title}
                  <span className="ml-auto rounded-full border px-1.5 font-mono text-[9px] uppercase text-muted-foreground">{t.priority}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      ))}
    </div>
  );
}

function CapturesPreview({ items, expiring }: { items: KnowledgeItem[] | null; expiring: { title: string; date: string; days_left: number }[] | null }) {
  const exp = expiring ?? [];
  return (
    <div className="space-y-0.5">
      {(items ?? []).slice(0, 4).map((i) => (
        <Row key={i.id} left={i.title} right={i.type} dim />
      ))}
      {(items ?? []).length === 0 && <p className="text-xs text-muted-foreground">Nothing captured yet.</p>}
      {exp.length > 0 && (
        <div className="mt-2 space-y-0.5 border-t pt-2">
          {exp.slice(0, 3).map((e, i) => (
            <Row key={i} left={`${e.title} · ${e.date}`} right={`${e.days_left}d`} />
          ))}
        </div>
      )}
    </div>
  );
}

function CapturesDetail({ reload }: { reload: () => void }) {
  const items = useTileData<{ items: KnowledgeItem[] }>("/v1/search?page_size=12", "universal");
  return (
    <div className="grid gap-4 overflow-y-auto pr-1 md:grid-cols-2" style={{ maxHeight: "calc(100vh - 12rem)" }}>
      <QuickAdd onAdded={reload} />
      <ul className="space-y-1.5">
        {(items?.items ?? []).map((i) => (
          <li key={i.id} className="rounded-md border p-2">
            <div className="flex items-center gap-2">
              <span className="rounded-full border px-1.5 font-mono text-[9px] uppercase text-muted-foreground">{i.type}</span>
              <span className="truncate text-sm">{i.title}</span>
            </div>
            {i.body && <p className="mt-0.5 truncate text-[11px] text-muted-foreground">{i.body}</p>}
          </li>
        ))}
      </ul>
    </div>
  );
}

function GroceryPreview({ items }: { items: GroceryItem[] | null }) {
  const pending = (items ?? []).filter((i) => !i.checked);
  return (
    <div className="flex items-center gap-4">
      <Metric label="Pending" value={String(pending.length)} strong />
      <div className="flex-1 space-y-0.5">
        {pending.slice(0, 3).map((i) => (
          <Row key={i.id} left={i.name} right={i.qty} dim />
        ))}
        {pending.length === 0 && <p className="text-xs text-muted-foreground">List clear.</p>}
      </div>
    </div>
  );
}

// ---- board ----

function Board() {
  const router = useRouter();
  const searchParams = useSearchParams();

  const [expanded, setExpanded] = useState<TileId | null>(() => {
    const e = searchParams.get("expand");
    return e && TILES.some((t) => t.id === e) ? (e as TileId) : null;
  });
  const [ringPillar, setRingPillar] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const bump = useCallback(() => setReloadKey((k) => k + 1), []);

  // Agent focus: CustomEvent (deep-link handled by lazy init above).
  useEffect(() => {
    const onFocus = (ev: Event) => {
      const id = (ev as CustomEvent).detail as TileId;
      if (TILES.some((t) => t.id === id)) setExpanded(id);
    };
    const onChanged = (ev: Event) => {
      const pillar = (ev as CustomEvent).detail as string;
      setRingPillar(pillar);
      setTimeout(() => setRingPillar(null), 2200);
    };
    window.addEventListener("personal-os:focus", onFocus);
    window.addEventListener("personal-os:changed", onChanged);
    return () => {
      window.removeEventListener("personal-os:focus", onFocus);
      window.removeEventListener("personal-os:changed", onChanged);
    };
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setExpanded(null);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  // Tile data (each re-fetches when its pillar version bumps).
  const plannerV = usePillarVersion("planner");
  const financeV = usePillarVersion("finance");
  const healthV = usePillarVersion("health");
  const universalV = usePillarVersion("universal");
  const today = useTileData<TodayBundle>("/v1/planner/today", "planner");
  const upcoming = useTileData<{ items: { date: string; tasks: Task[]; events: Occurrence[] }[] }>(
    "/v1/planner/upcoming?days=7",
    "planner",
  );
   
  const month = currentMonth();
  const money = useTileData<MonthSummary>(`/v1/finance/summary?month=${month}`, "finance");
  const health = useTileData<HealthSummary>(
    `/v1/health/summary?from=${isoDaysAgo(13)}&to=${todayStr()}`,
    "health",
  );
  const captures = useTileData<{ items: KnowledgeItem[] }>("/v1/search?page_size=5", "universal");
  const expiring = useTileData<{ items: { title: string; date: string; days_left: number }[] }>(
    "/v1/items/expiring?days=30",
    "universal",
  );
  const grocery = useTileData<{ items: GroceryItem[] }>("/v1/grocery", "health");

  const versionByPillar: Record<string, string> = {
    planner: plannerV ?? "init",
    finance: financeV ?? "init",
    health: healthV ?? "init",
    universal: universalV ?? "init",
    knowledge: "init",
  };

  const tilePreview: Record<TileId, React.ReactNode> = {
    today: <TodayPreview data={today} />,
    money: <MoneyPreview s={money} />,
    body: <BodyPreview s={health} />,
    upcoming: <UpcomingPreview data={upcoming} />,
    captures: <CapturesPreview items={captures?.items ?? null} expiring={expiring?.items ?? null} />,
    grocery: <GroceryPreview items={grocery?.items ?? null} />,
  };

  const tileDetail: Record<TileId, React.ReactNode> = {
    today: <TodayDetail data={today} reload={bump} />,
    money: <MoneyDetail month={month} />,
    body: <BodyDetail />,
    upcoming: <UpcomingDetail data={upcoming} />,
    captures: <CapturesDetail reload={bump} />,
    grocery: <GroceryChecklist reloadKey={reloadKey} onChanged={bump} />,
  };

  const tileSpan: Record<TileId, string> = {
    today: "col-span-6 sm:col-span-3 lg:col-span-2",
    money: "col-span-6 sm:col-span-3 lg:col-span-2",
    body: "col-span-6 sm:col-span-6 lg:col-span-2",
    upcoming: "col-span-6 lg:col-span-4",
    captures: "col-span-6 lg:col-span-2",
    grocery: "col-span-6",
  };

  const expand = (id: TileId) => {
    setExpanded(id);
    router.replace(`/?expand=${id}`, { scroll: false });
  };
  const collapse = () => {
    setExpanded(null);
    router.replace("/", { scroll: false });
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-sm font-semibold tracking-tight">Overview</h1>
        <span className="text-[10px] text-muted-foreground">click a card to expand · esc to close</span>
      </div>

      <div className="grid grid-cols-6 gap-4">
        {TILES.map(({ id, pillar }) => {
          const isExpanded = expanded === id;
          const dimmed = expanded !== null && !isExpanded;
          const ring = ringPillar === pillar && !isExpanded ? "ring-pulse" : "";
          return (
            <Card
              key={id + (versionByPillar[pillar] ?? "")}
              className={`board-tile tile-flash cursor-pointer hover:border-foreground/40 ${tileSpan[id]} ${
                dimmed ? "board-dimmed" : ""
              } ${ring}`}
              onClick={() => !expanded && expand(id)}
            >
              <CardContent className="p-4">
                <div className="value-in">
                  <div className="mb-3 flex items-center justify-between">
                    <h2 className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">{id}</h2>
                  </div>
                  {tilePreview[id]}
                </div>
              </CardContent>
            </Card>
          );
        })}
      </div>

      {/* Expanded detail overlay */}
      {expanded && (
        <>
          <div
            className="fixed inset-0 z-30 bg-background/70 backdrop-blur-[2px]"
            onClick={collapse}
          />
          <Card className="overlay-in fixed inset-4 z-40 mx-auto max-w-5xl overflow-hidden md:inset-10">
            <CardContent className="flex h-full flex-col p-5">
              <div className="mb-3 flex items-center justify-between">
                <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                  {expanded}
                </h2>
                <button
                  onClick={collapse}
                  className="rounded-md border p-1 text-muted-foreground hover:text-foreground"
                  aria-label="Close"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
              <div className="min-h-0 flex-1">{tileDetail[expanded]}</div>
            </CardContent>
          </Card>
        </>
      )}
    </div>
  );
}

export default function OverviewPage() {
  return (
    <Suspense fallback={null}>
      <Board />
    </Suspense>
  );
}
