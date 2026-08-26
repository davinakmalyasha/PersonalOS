"use client";

import { useCallback, useEffect, useState } from "react";
import { CalendarDays, History } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { apiGet, apiSend, todayStr } from "@/lib/api";

type Note = {
  id: string;
  title: string;
  body: string;
  tags: string[];
  updated_at: string;
};

export function DailyNoteCard({ onChanged }: { onChanged?: () => void }) {
  const [note, setNote] = useState<Note | null>(null);
  const [created, setCreated] = useState(false);
  const [text, setText] = useState("");

  const load = useCallback(async () => {
    try {
      const res = await apiGet<{ note: Note; created: boolean }>("/v1/knowledge/daily");
      setNote(res.note);
      setCreated(res.created);
    } catch {
      /* non-fatal */
    }
  }, []);
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const append = async () => {
    if (!text.trim()) return;
    try {
      const updated = await apiSend<Note>("/v1/knowledge/daily", "PATCH", { text: text.trim() });
      setNote(updated);
      setText("");
      onChanged?.();
    } catch {
      /* non-fatal */
    }
  };

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle className="flex items-center gap-2 text-sm">
          <CalendarDays className="h-3.5 w-3.5" /> Daily note
        </CardTitle>
        {created && <span className="font-mono text-[9px] uppercase text-muted-foreground">new</span>}
      </CardHeader>
      <CardContent className="space-y-2">
        <pre className="max-h-40 overflow-y-auto whitespace-pre-wrap rounded-md border bg-muted/30 p-2.5 font-sans text-xs leading-5">
          {note?.body?.trim() || "Empty — add your first line below."}
        </pre>
        <div className="flex gap-1.5">
          <input
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && void append()}
            placeholder={`Add a line to ${todayStr()}…`}
            className="h-8 flex-1 rounded-md border border-input bg-background px-2 text-xs"
          />
          <Button size="sm" variant="secondary" onClick={() => void append()}>
            Append
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

type Resurfaced = {
  kind: string;
  id: string;
  title: string;
  url?: string | null;
  year: number;
  snippet?: string;
};

type Highlight = {
  id: string;
  reading_id: string;
  quote: string;
  note: string;
  interval_days: number;
};

// Spaced-repetition review queue (Readwise-style daily digest).
export function HighlightsDueCard({ onChanged }: { onChanged?: () => void }) {
  const [items, setItems] = useState<Highlight[]>([]);

  const load = useCallback(() => {
    apiGet<{ items: Highlight[] }>("/v1/knowledge/highlights/due")
      .then((r) => setItems(r.items ?? []))
      .catch(() => {});
  }, []);
  useEffect(() => {
    load();
  }, [load]);

  const review = async (id: string, remembered: boolean) => {
    await apiSend(`/v1/highlights/${id}/review`, "POST", { remembered });
    load();
    onChanged?.();
  };

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle className="flex items-center gap-2 text-sm">
          <History className="h-3.5 w-3.5" /> Review queue
        </CardTitle>
        <span className="font-mono text-[10px] text-muted-foreground">{items.length} due</span>
      </CardHeader>
      <CardContent>
        {items.length === 0 ? (
          <p className="rounded-md border border-dashed p-3 text-center text-xs text-muted-foreground">
            Nothing to review — highlights land here on a spaced schedule.
          </p>
        ) : (
          <ul className="space-y-2">
            {items.slice(0, 3).map((hh) => (
              <li key={hh.id} className="rounded-md border p-2.5 text-xs">
                <p className="leading-5">“{hh.quote}”</p>
                {hh.note && <p className="mt-1 text-[11px] text-muted-foreground">{hh.note}</p>}
                <div className="mt-1.5 flex gap-1.5">
                  <Button size="sm" variant="outline" className="h-6 px-2 text-[10px]" onClick={() => void review(hh.id, true)}>
                    remembered
                  </Button>
                  <Button size="sm" variant="ghost" className="h-6 px-2 text-[10px] text-muted-foreground" onClick={() => void review(hh.id, false)}>
                    forgot
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

export function ResurfaceStrip() {
  const [items, setItems] = useState<Resurfaced[]>([]);
  useEffect(() => {
    apiGet<{ items: Resurfaced[] }>("/v1/knowledge/resurface")
      .then((r) => setItems(r.items ?? []))
      .catch(() => {});
  }, []);

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle className="flex items-center gap-2 text-sm">
          <History className="h-3.5 w-3.5" /> On this day
        </CardTitle>
        <span className="font-mono text-[10px] text-muted-foreground">{todayStr().slice(5)}</span>
      </CardHeader>
      <CardContent>
        {items.length === 0 ? (
          <p className="rounded-md border border-dashed p-3 text-center text-xs text-muted-foreground">
            Nothing from earlier years — yet.
          </p>
        ) : (
          <ul className="space-y-1.5">
            {items.slice(0, 5).map((r) => (
              <li key={r.kind + r.id} className="flex items-start gap-2 text-xs">
                <span className="shrink-0 rounded-full border px-1.5 font-mono text-[9px] uppercase text-muted-foreground">
                  {r.year}
                </span>
                <div className="min-w-0">
                  <p className="truncate">{r.title}</p>
                  {r.snippet && <p className="truncate text-[11px] text-muted-foreground">{r.snippet}</p>}
                </div>
                <span className="ml-auto shrink-0 font-mono text-[9px] uppercase text-muted-foreground">{r.kind}</span>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
