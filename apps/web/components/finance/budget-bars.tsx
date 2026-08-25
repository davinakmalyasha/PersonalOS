import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { formatMinor } from "@/lib/api";
import type { MonthSummary } from "@/lib/api";

export function BudgetBars({ budgets }: { budgets: MonthSummary["budgets"] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Budgets</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {budgets.length === 0 && (
          <p className="text-sm text-muted-foreground">
            No budgets for this month. POST /v1/budgets to set one.
          </p>
        )}
        {budgets.map((b) => {
          const pct = b.budget_minor > 0 ? Math.min(100, (b.spent_minor / b.budget_minor) * 100) : 0;
          return (
            <div key={b.category_id} className="space-y-1.5">
              <div className="flex items-baseline justify-between text-sm">
                <span>{b.category_name}</span>
                <span className="font-mono text-xs tabular-nums text-muted-foreground">
                  {formatMinor(b.spent_minor)} / {formatMinor(b.budget_minor)}
                  {b.over && <span className="ml-2 font-semibold text-destructive">over</span>}
                </span>
              </div>
              <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                <div
                  className={`h-full rounded-full ${b.over ? "bg-destructive" : "bg-chart-3"}`}
                  style={{ width: `${pct}%` }}
                />
              </div>
            </div>
          );
        })}
      </CardContent>
    </Card>
  );
}
