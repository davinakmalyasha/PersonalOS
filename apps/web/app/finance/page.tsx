"use client";

import { useCallback, useEffect, useState } from "react";
import { CategoryChart } from "@/components/finance/category-chart";
import { BudgetBars } from "@/components/finance/budget-bars";
import { SummaryCards } from "@/components/finance/summary-cards";
import { TransactionsTable } from "@/components/finance/transactions-table";
import { ImportPanel } from "@/components/finance/import-panel";
import {
  AliasesPanel,
  BillsStrip,
  ForecastCard,
  GoalsPanel,
  SafeToSpendCard,
  SubscriptionsPanel,
} from "@/components/finance/extras-panels";
import {
  apiGet,
  currentMonth,
  type Account,
  type MonthSummary,
} from "@/lib/api";

export default function FinancePage() {
  const [month, setMonth] = useState(currentMonth());
  const [summary, setSummary] = useState<MonthSummary | null>(null);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [error, setError] = useState("");

  const reload = useCallback(async () => {
    try {
      const [s, a] = await Promise.all([
        apiGet<MonthSummary>(`/v1/finance/summary?month=${month}`),
        apiGet<{ items: Account[] }>("/v1/accounts"),
      ]);
      setSummary(s);
      setAccounts(a.items ?? []);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [month]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void reload();
  }, [reload]);

  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="sticky top-0 z-10 border-b bg-background/80 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-3">
          <span className="text-sm font-semibold tracking-tight">Finance</span>
          <div className="flex items-center gap-2">
            <input
              type="month"
              value={month}
              onChange={(e) => setMonth(e.target.value || currentMonth())}
              className="h-8 rounded-md border border-input bg-background px-2 font-mono text-xs"
            />
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-6xl space-y-6 px-6 py-8">
        {error && (
          <p className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">
            API unreachable: {error}. Start it with{" "}
            <code className="font-mono">go run ./services/api/cmd/api</code>.
          </p>
        )}

        <BillsStrip />

        <div className="grid gap-4 lg:grid-cols-2">
          <SafeToSpendCard />
          <ForecastCard />
        </div>

        <SummaryCards
          income={summary?.income_minor ?? 0}
          outcome={summary?.outcome_minor ?? 0}
          net={summary?.net_minor ?? 0}
        />

        <div className="grid gap-4 lg:grid-cols-5">
          <div className="lg:col-span-3">
            <CategoryChart data={summary?.by_category ?? []} />
          </div>
          <div className="lg:col-span-2">
            <BudgetBars budgets={summary?.budgets ?? []} />
          </div>
        </div>

        <div className="grid gap-4 lg:grid-cols-3">
          <GoalsPanel />
          <SubscriptionsPanel />
          <AliasesPanel onChanged={() => void reload()} />
        </div>

        <ImportPanel accounts={accounts.map((a) => ({ id: a.id, name: a.name }))} onImported={() => void reload()} />

        <TransactionsTable accountId={accounts[0]?.id ?? ""} />
      </main>
    </div>
  );
}
