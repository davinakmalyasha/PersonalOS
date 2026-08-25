import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { formatMinor } from "@/lib/api";

export function SummaryCards({
  income,
  outcome,
  net,
  currency = "IDR",
}: {
  income: number;
  outcome: number;
  net: number;
  currency?: string;
}) {
  return (
    <div className="grid gap-4 sm:grid-cols-3">
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-xs uppercase tracking-wide text-muted-foreground">Income</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="font-mono text-lg tabular-nums">{formatMinor(income, currency)}</div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-xs uppercase tracking-wide text-muted-foreground">Outcome</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="font-mono text-lg tabular-nums">{formatMinor(outcome, currency)}</div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-xs uppercase tracking-wide text-muted-foreground">Net</CardTitle>
        </CardHeader>
        <CardContent>
          <div
            className={`font-mono text-lg tabular-nums ${net < 0 ? "text-destructive" : ""}`}
          >
            {formatMinor(net, currency)}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
