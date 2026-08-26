"use client";

import { useCallback, useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { SearchPanel } from "@/components/knowledge/search-panel";
import { QuickAdd } from "@/components/knowledge/quick-add";
import { ReadingBoard } from "@/components/knowledge/reading-board";
import { DailyNoteCard, HighlightsDueCard, ResurfaceStrip } from "@/components/knowledge/memory-panels";

export default function KnowledgePage() {
  const [reloadKey, setReloadKey] = useState(0);
  const [apiError, setApiError] = useState("");

  const reload = useCallback(() => setReloadKey((k) => k + 1), []);

  useEffect(() => {
    // Health probe so the banner shows when the Go API is down.
    fetch(`${process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"}/healthz`)
      .then((r) => {
        if (!r.ok) throw new Error(String(r.status));
        setApiError("");
      })
      .catch((e) => setApiError(e instanceof Error ? e.message : String(e)));
  }, []);

  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="sticky top-0 z-10 border-b bg-background/80 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-3">
          <span className="text-sm font-semibold tracking-tight">Knowledge</span>
        </div>
      </header>

      <main className="mx-auto max-w-6xl space-y-6 px-6 py-8">
        {apiError && (
          <p className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">
            API unreachable: {apiError}. Start it with{" "}
            <code className="font-mono">go run ./services/api/cmd/api</code>.
          </p>
        )}

        <div className="grid gap-4 lg:grid-cols-5">
          <section className="space-y-4 lg:col-span-2">
            <Card>
              <CardHeader>
                <CardTitle className="text-sm">Capture</CardTitle>
              </CardHeader>
              <CardContent>
                <QuickAdd onAdded={reload} />
              </CardContent>
            </Card>
            <DailyNoteCard />
            <HighlightsDueCard />
            <ResurfaceStrip />
          </section>

          <section className="lg:col-span-3">
            <SearchPanel reloadKey={reloadKey} />
          </section>
        </div>

        <ReadingBoard reloadKey={reloadKey} onChanged={reload} />
      </main>
    </div>
  );
}
