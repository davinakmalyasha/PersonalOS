"use client";

import { useCallback, useEffect, useState } from "react";
import { Star } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { apiGet, apiSend, type ReadingEntry } from "@/lib/api";

const COLUMNS: { status: ReadingEntry["status"]; label: string; next?: ReadingEntry["status"] }[] = [
  { status: "to-read", label: "To read", next: "reading" },
  { status: "reading", label: "Reading", next: "done" },
  { status: "done", label: "Done" },
];

function Stars({ rating }: { rating: number | null }) {
  if (!rating) return null;
  return (
    <span className="flex items-center gap-0.5">
      {Array.from({ length: 5 }).map((_, i) => (
        <Star key={i} className={`h-3 w-3 ${i < rating ? "fill-foreground text-foreground" : "text-muted-foreground/40"}`} />
      ))}
    </span>
  );
}

export function ReadingBoard({ reloadKey, onChanged }: { reloadKey: number; onChanged: () => void }) {
  const [entries, setEntries] = useState<ReadingEntry[]>([]);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const res = await apiGet<{ items: ReadingEntry[] }>("/v1/reading?page_size=100");
      setEntries(res.items ?? []);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load, reloadKey]);

  const advance = async (rd: ReadingEntry, next: ReadingEntry["status"]) => {
    await apiSend(`/v1/reading/${rd.id}`, "PATCH", { status: next });
    await load();
    onChanged();
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Reading list</CardTitle>
      </CardHeader>
      <CardContent>
        {error && <p className="text-sm text-destructive">{error}</p>}
        {!error && (
          <div className="grid gap-3 sm:grid-cols-3">
            {COLUMNS.map((col) => {
              const items = entries.filter((e) => e.status === col.status);
              return (
                <div key={col.status} className="rounded-md border p-2.5">
                  <div className="mb-2 flex items-center justify-between">
                    <h4 className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">{col.label}</h4>
                    <Badge variant="outline" className="font-mono text-[10px]">{items.length}</Badge>
                  </div>
                  <ul className="space-y-1.5">
                    {items.map((rd) => (
                      <li key={rd.id} className="rounded-md border bg-background p-2">
                        <p className="truncate text-xs font-medium">{rd.title}</p>
                        {rd.author && <p className="truncate text-[10px] text-muted-foreground">{rd.author}</p>}
                        <div className="mt-1 flex items-center justify-between gap-1">
                          <Stars rating={rd.rating} />
                          {col.next ? (
                            <Button size="sm" variant="outline" className="h-5 px-1.5 text-[10px]" onClick={() => void advance(rd, col.next!)}>
                              → {COLUMNS.find((c) => c.status === col.next)?.label}
                            </Button>
                          ) : null}
                        </div>
                      </li>
                    ))}
                    {items.length === 0 && (
                      <li className="rounded-md border border-dashed p-2 text-center text-[10px] text-muted-foreground">—</li>
                    )}
                  </ul>
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
