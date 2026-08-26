"use client";

import { useCallback, useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { apiGet, apiSend, formatMinor, type UpcomingBill } from "@/lib/api";

type Goal = {
  id: string;
  kind: string;
  name: string;
  target_minor: number | null;
  saved_minor: number;
  deadline: string | null;
};

type RecurringSub = {
  merchant: string;
  amount_minor: number;
  occurrences: number;
  last_date?: string;
  next_guess?: string;
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

export function SubscriptionsPanel() {
  const [subs, setSubs] = useState<RecurringSub[]>([]);
  useEffect(() => {
    apiGet<{ items: RecurringSub[] }>("/v1/finance/recurring")
      .then((r) => setSubs(r.items ?? []))
      .catch(() => {});
  }, []);
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Detected subscriptions</CardTitle>
      </CardHeader>
      <CardContent>
        {subs.length === 0 ? (
          <p className="rounded-md border border-dashed p-3 text-center text-xs text-muted-foreground">
            Nothing recurring detected yet.
          </p>
        ) : (
          <ul className="divide-y text-xs">
            {subs.map((s) => (
              <li key={s.merchant + s.amount_minor} className="flex items-center justify-between gap-2 py-1.5 first:pt-0 last:pb-0">
                <span className="truncate">{s.merchant}</span>
                <span className="shrink-0 font-mono tabular-nums text-muted-foreground">
                  {formatMinor(s.amount_minor)} · ×{s.occurrences}
                  {s.next_guess ? ` · next ${s.next_guess}` : ""}
                </span>
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
