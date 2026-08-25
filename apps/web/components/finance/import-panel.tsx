"use client";

import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { API_BASE, type ImportResult } from "@/lib/api";

export function ImportPanel({
  accounts,
  onImported,
}: {
  accounts: { id: string; name: string }[];
  onImported: () => void;
}) {
  const [accountId, setAccountId] = useState("");
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [error, setError] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);

  async function submit() {
    if (!accountId || !fileRef.current?.files?.[0]) return;
    setBusy(true);
    setError("");
    setResult(null);
    try {
      const form = new FormData();
      form.set("account_id", accountId);
      form.set("file", fileRef.current.files[0]);
      const res = await fetch(`${API_BASE}/v1/transactions/import`, {
        method: "POST",
        body: form,
      });
      if (!res.ok) {
        throw new Error(await res.text());
      }
      const json = (await res.json()) as ImportResult;
      setResult(json);
      if (fileRef.current) fileRef.current.value = "";
      onImported();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Import bank CSV</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={accountId}
            onChange={(e) => setAccountId(e.target.value)}
            className="h-8 rounded-md border border-input bg-background px-2 text-xs"
          >
            <option value="">Select account…</option>
            {accounts.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </select>
          <input
            ref={fileRef}
            type="file"
            accept=".csv,text/csv"
            className="text-xs text-muted-foreground file:mr-2 file:h-7 file:rounded-md file:border file:border-input file:bg-background file:px-2 file:text-xs"
          />
          <Button size="sm" disabled={busy || !accountId} onClick={() => void submit()}>
            {busy ? "Importing…" : "Import"}
          </Button>
        </div>
        <p className="text-xs text-muted-foreground">
          Columns auto-detected (English + Indonesian headers). Duplicates by date+amount+description
          are skipped — re-importing the same file is always safe.
        </p>
        {error && <p className="whitespace-pre-wrap text-xs text-destructive">{error}</p>}
        {result && (
          <div className="rounded-md border bg-muted/40 p-3 font-mono text-xs">
            imported: {result.imported} · duplicates skipped: {result.skipped} · invalid:{" "}
            {result.skipped_invalid} · auto-categorized: {result.auto_categorized}
            {result.errors && result.errors.length > 0 && (
              <ul className="mt-2 list-inside list-disc text-destructive">
                {result.errors.slice(0, 5).map((e, i) => (
                  <li key={i}>
                    line {e.line}: {e.message}
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
