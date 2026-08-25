"use client";

import { useCallback, useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { apiGet, formatMinor, type Transaction } from "@/lib/api";

export function TransactionsTable({ accountId }: { accountId: string }) {
  const [q, setQ] = useState("");
  const [cat, setCat] = useState("");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<{ items: Transaction[]; total: number; page_size: number } | null>(
    null,
  );
  const [error, setError] = useState("");

  const load = useCallback(
    async (p = page, query = q, category = cat) => {
      try {
        const params = new URLSearchParams({
          account_id: accountId,
          page: String(p),
          page_size: "20",
        });
        if (query) params.set("q", query);
        if (category) params.set("category_id", category);
        const res = await apiGet<{ items: Transaction[]; total: number; page_size: number }>(
          `/v1/transactions?${params.toString()}`,
        );
        setData(res);
        setError("");
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      }
    },
    [accountId, page, q, cat],
  );

  useEffect(() => {
    if (!accountId) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load(page);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [accountId, page]);

  const totalPages = data ? Math.max(1, Math.ceil(data.total / (data.page_size || 20))) : 1;

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle className="text-sm">Transactions</CardTitle>
        <div className="flex gap-2">
          <Input
            placeholder="Search merchant…"
            value={q}
            onChange={(e) => {
              setQ(e.target.value);
              setPage(1);
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter") void load(1);
            }}
            className="h-8 w-48"
          />
          <Button size="sm" variant="secondary" onClick={() => void load(1)}>
            Search
          </Button>
          <select
            value={cat}
            onChange={(e) => {
              setCat(e.target.value);
              void load(1, q, e.target.value);
            }}
            className="h-8 rounded-md border border-input bg-background px-2 text-xs"
          >
            <option value="">All categories</option>
            <option value="none">Uncategorized</option>
          </select>
        </div>
      </CardHeader>
      <CardContent>
        {error && <p className="text-sm text-destructive">{error}</p>}
        {!error && (
          <div className="overflow-x-auto rounded-md border">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-muted/60 text-left text-xs uppercase tracking-wide text-muted-foreground">
                  <th className="px-3 py-2 font-medium">Date</th>
                  <th className="px-3 py-2 font-medium">Merchant</th>
                  <th className="px-3 py-2 font-medium">Category</th>
                  <th className="px-3 py-2 text-right font-medium">Amount</th>
                </tr>
              </thead>
              <tbody>
                {(data?.items ?? []).map((t) => (
                  <tr key={t.id} className="border-t">
                    <td className="px-3 py-2 font-mono text-xs tabular-nums">{t.date}</td>
                    <td className="px-3 py-2">{t.merchant}</td>
                    <td className="px-3 py-2">
                      {t.category_name ? (
                        <Badge variant="secondary">{t.category_name}</Badge>
                      ) : (
                        <Badge variant="outline" className="text-muted-foreground">
                          —
                        </Badge>
                      )}
                    </td>
                    <td
                      className={`px-3 py-2 text-right font-mono tabular-nums ${
                        t.amount_minor < 0 ? "" : "text-muted-foreground"
                      }`}
                    >
                      {formatMinor(t.amount_minor, t.currency)}
                    </td>
                  </tr>
                ))}
                {data?.items.length === 0 && (
                  <tr>
                    <td colSpan={4} className="px-3 py-6 text-center text-muted-foreground">
                      No transactions match.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        )}
        <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground">
          <span>
            {data ? `${data.total} total` : ""}
          </span>
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="outline"
              disabled={page <= 1}
              onClick={() => {
                const p = page - 1;
                setPage(p);
                void load(p);
              }}
            >
              Prev
            </Button>
            <span>
              {page} / {totalPages}
            </span>
            <Button
              size="sm"
              variant="outline"
              disabled={page >= totalPages}
              onClick={() => {
                const p = page + 1;
                setPage(p);
                void load(p);
              }}
            >
              Next
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
