"use client";

import { useCallback, useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  apiGet,
  apiSend,
  formatMinor,
  todayStr,
  type Forecast,
  type SafeToSpend,
  type Subscription,
  type UpcomingBill,
} from "@/lib/api";

type Goal = {
  id: string;
  kind: string;
  name: string;
  target_minor: number | null;
  saved_minor: number;
  deadline: string | null;
};

type Alias = { id: string; pattern: string; canonical: string };

export function BillsStrip() {
  const [bills, setBills] = useState<UpcomingBill[]>([]);
  useEffect(() => {
    apiGet<{ items: UpcomingBill[] }>("/v1/finance/bills?days=7")
      .then((r) => setBills(r.items ?? []))
      .catch(() => {});
  }, []);
  if (bills.length === 0) return null;
  return (
    <div className="flex flex-wrap items-center gap-2 rounded-md border p-3">
      <span className="text-[10px] uppercase tracking-wide text-muted-foreground">Due ≤7d</span>
      {bills.slice(0, 5).map((b) => (
        <span key={b.merchant + b.next_guess} className="flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs">
          <span className="max-w-[140px] truncate">{b.merchant}</span>
          <span className="font-mono tabular-nums text-muted-foreground">{formatMinor(b.amount_minor)}</span>
          <span className="font-mono text-[10px] tabular-nums text-muted-foreground">
            {b.days_left <= 0 ? "today" : `${b.days_left}d`}
          </span>
        </span>
      ))}
    </div>
  );
}

export function GoalsPanel() {
  const [goals, setGoals] = useState<Goal[]>([]);
  const reload = useCallback(() => {
    apiGet<{ items: Goal[] }>("/v1/goals?kind=savings")
      .then((r) => setGoals(r.items ?? []))
      .catch(() => {});
  }, []);
  useEffect(reload, [reload]);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Savings goals</CardTitle>
      </CardHeader>
      <CardContent>
        {goals.length === 0 ? (
          <p className="rounded-md border border-dashed p-3 text-center text-xs text-muted-foreground">
            No savings goals yet.
          </p>
        ) : (
          <ul className="space-y-2">
            {goals.map((g) => {
              const pct = g.target_minor ? Math.min(100, Math.round((g.saved_minor / g.target_minor) * 100)) : 0;
              return (
                <li key={g.id} className="space-y-1">
                  <div className="flex items-center justify-between gap-2 text-xs">
                    <span className="truncate">{g.name}</span>
                    <span className="shrink-0 font-mono tabular-nums text-muted-foreground">
                      {formatMinor(g.saved_minor)}
                      {g.target_minor ? ` / ${formatMinor(g.target_minor)}` : ""} · {pct}%
                    </span>
                  </div>
                  <div className="h-1 overflow-hidden rounded-full bg-muted">
                    <div className="h-full rounded-full bg-foreground" style={{ width: `${pct}%` }} />
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

export function SafeToSpendCard() {
  const [sts, setSts] = useState<SafeToSpend | null>(null);
  useEffect(() => {
    apiGet<SafeToSpend>("/v1/finance/safe-to-spend")
      .then(setSts)
      .catch(() => {});
  }, []);
  return (
    <Card>
      <CardContent className="flex items-center justify-between py-4">
        <div>
          <p className="text-[10px] uppercase tracking-wide text-muted-foreground">
            Safe to spend · {sts?.month ?? todayStr().slice(0, 7)}
          </p>
          <p className="font-mono text-2xl font-semibold tabular-nums">
            {sts ? formatMinor(sts.safe_to_spend_minor) : "—"}
          </p>
        </div>
        <div className="space-y-0.5 text-right font-mono text-[11px] tabular-nums text-muted-foreground">
          <p>in {sts ? formatMinor(sts.income_mtd_minor) : "—"}</p>
          <p>out {sts ? formatMinor(sts.spend_mtd_minor) : "—"}</p>
          <p>budget left {sts ? formatMinor(sts.budget_left_minor) : "—"}</p>
          <p>bills ahead {sts ? formatMinor(sts.bills_ahead_minor) : "—"}</p>
        </div>
      </CardContent>
    </Card>
  );
}

export function ForecastCard() {
  const [fc, setFc] = useState<Forecast | null>(null);
  useEffect(() => {
    apiGet<Forecast>("/v1/finance/forecast?days=30")
      .then(setFc)
      .catch(() => {});
  }, []);
  if (!fc || fc.points.length === 0) return null;
  const vals = fc.points.map((p) => p.projected_minor);
  const min = Math.min(...vals);
  const max = Math.max(...vals);
  const span = max > min ? max - min : 1;
  const pts = fc.points
    .map((p, i) => `${(i / (fc.points.length - 1)) * 100},${28 - ((p.projected_minor - min) / span) * 26}`)
    .join(" ");
  return (
    <Card>
      <CardHeader className="pb-0">
        <CardTitle className="text-sm">Cash-flow forecast · 30d</CardTitle>
      </CardHeader>
      <CardContent>
        <svg viewBox="0 0 100 30" preserveAspectRatio="none" className="h-12 w-full">
          <polyline points={pts} fill="none" stroke="currentColor" strokeWidth="1.2" vectorEffect="non-scaling-stroke" />
        </svg>
        <div className="mt-1 flex items-center justify-between font-mono text-[10px] tabular-nums text-muted-foreground">
          <span>now {formatMinor(fc.start_minor)}</span>
          {fc.lowest && (
            <span className={fc.lowest.projected_minor < 0 ? "text-destructive" : ""}>
              low {formatMinor(fc.lowest.projected_minor)} on {fc.lowest.date}
            </span>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

export function SubscriptionsPanel() {
  const [subs, setSubs] = useState<Subscription[]>([]);
  const [showAll, setShowAll] = useState(false);

  const reload = useCallback(() => {
    apiGet<{ items: Subscription[] }>("/v1/subscriptions")
      .then((r) => setSubs(r.items ?? []))
      .catch(() => {});
  }, []);
  useEffect(reload, [reload]);

  const sync = async () => {
    await apiSend("/v1/finance/subscriptions/sync", "POST");
    reload();
  };
  const setStatus = async (id: string, status: Subscription["status"]) => {
    await apiSend(`/v1/subscriptions/${id}`, "PATCH", { status });
    reload();
  };

  const visible = subs.filter((s) => showAll || s.status === "active");
  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle className="text-sm">Subscriptions</CardTitle>
        <div className="flex gap-1.5">
          <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px]" onClick={() => void sync()}>
            re-detect
          </Button>
          <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px]" onClick={() => setShowAll(!showAll)}>
            {showAll ? "active only" : "show all"}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {visible.length === 0 ? (
          <p className="rounded-md border border-dashed p-3 text-center text-xs text-muted-foreground">
            No subscriptions yet — import statements then hit “re-detect”.
          </p>
        ) : (
          <ul className="divide-y text-xs">
            {visible.map((s) => (
              <li key={s.id} className="flex items-center justify-between gap-2 py-1.5 first:pt-0 last:pb-0">
                <div className="min-w-0">
                  <p className="truncate">
                    {s.merchant}
                    {s.status !== "active" && (
                      <span className="ml-1.5 rounded-full border px-1.5 font-mono text-[9px] uppercase text-muted-foreground">
                        {s.status}
                      </span>
                    )}
                  </p>
                  <p className="font-mono text-[10px] tabular-nums text-muted-foreground">
                    ×{s.occurrences}
                    {s.next_due ? ` · next ${s.next_due}` : ""}
                  </p>
                </div>
                <div className="flex shrink-0 items-center gap-1.5">
                  <span className="font-mono tabular-nums">{formatMinor(s.amount_minor)}</span>
                  {s.status === "active" ? (
                    <>
                      <Button variant="outline" size="sm" className="h-6 px-2 text-[10px]" onClick={() => void setStatus(s.id, "muted")}>
                        mute
                      </Button>
                      <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px] text-destructive" onClick={() => void setStatus(s.id, "cancelled")}>
                        cancel
                      </Button>
                    </>
                  ) : (
                    <Button variant="outline" size="sm" className="h-6 px-2 text-[10px]" onClick={() => void setStatus(s.id, "active")}>
                      restore
                    </Button>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

export function AliasesPanel({ onChanged }: { onChanged?: () => void }) {
  const [aliases, setAliases] = useState<Alias[]>([]);
  const [pattern, setPattern] = useState("");
  const [canonical, setCanonical] = useState("");

  const reload = useCallback(() => {
    apiGet<{ items: Alias[] }>("/v1/merchant_aliases")
      .then((r) => setAliases(r.items ?? []))
      .catch(() => {});
  }, []);
  useEffect(reload, [reload]);

  const create = async () => {
    if (!pattern.trim() || !canonical.trim()) return;
    await apiSend("/v1/merchant_aliases", "POST", { pattern: pattern.trim(), canonical: canonical.trim() });
    setPattern("");
    setCanonical("");
    reload();
    onChanged?.();
  };
  const remove = async (id: string) => {
    await apiSend(`/v1/merchant_aliases/${id}`, "DELETE");
    reload();
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Merchant aliases</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <ul className="divide-y text-xs">
          {aliases.map((a) => (
            <li key={a.id} className="flex items-center justify-between gap-2 py-1.5 first:pt-0 last:pb-0">
              <span className="truncate font-mono text-[11px] text-muted-foreground">
                {a.pattern} → <span className="text-foreground">{a.canonical}</span>
              </span>
              <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px]" onClick={() => void remove(a.id)}>
                remove
              </Button>
            </li>
          ))}
        </ul>
        <div className="flex items-center gap-2">
          <input
            value={pattern}
            onChange={(e) => setPattern(e.target.value)}
            placeholder="bank text…"
            className="h-8 flex-1 rounded-md border border-input bg-background px-2 font-mono text-xs"
          />
          <span className="text-muted-foreground">→</span>
          <input
            value={canonical}
            onChange={(e) => setCanonical(e.target.value)}
            placeholder="clean name"
            className="h-8 w-28 rounded-md border border-input bg-background px-2 font-mono text-xs"
          />
          <Button size="sm" variant="secondary" onClick={() => void create()}>
            Add
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
