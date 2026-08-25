"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ThemeToggle } from "@/components/theme-toggle";
import { SearchPanel } from "@/components/knowledge/search-panel";
import { QuickAdd } from "@/components/knowledge/quick-add";
import { ReadingBoard } from "@/components/knowledge/reading-board";

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
          <div className="flex items-center gap-3">
            <Link href="/" className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
              <ArrowLeft className="h-4 w-4" /> Home
            </Link>
            <span className="text-sm font-semibold tracking-tight">Knowledge</span>
          </div>
          <ThemeToggle />
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
          <section className="lg:col-span-2 space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-sm">Capture</CardTitle>
              </CardHeader>
              <CardContent>
                <QuickAdd onAdded={reload} />
              </CardContent>
            </Card>
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
