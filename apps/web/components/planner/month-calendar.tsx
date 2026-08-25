"use client";

import { useMemo } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { Occurrence, Task } from "@/lib/api";

type DayCell = {
  date: string; // YYYY-MM-DD
  day: number;
  inMonth: boolean;
};

const WEEKDAYS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

function fmt(y: number, m: number, d: number): string {
  return `${y}-${String(m + 1).padStart(2, "0")}-${String(d).padStart(2, "0")}`;
}

export function MonthCalendar({
  month, // YYYY-MM
  selected,
  onSelect,
  events,
  tasksDue,
}: {
  month: string;
  selected: string;
  onSelect: (date: string) => void;
  events: Occurrence[];
  tasksDue: Task[]; // open tasks with a due_date (any month)
}) {
  const [year, monthIdx] = month.split("-").map(Number);

  const cells = useMemo<DayCell[]>(() => {
    const first = new Date(Date.UTC(year, monthIdx - 1, 1));
    const startOffset = (first.getUTCDay() + 6) % 7; // Monday-first
    const daysInMonth = new Date(Date.UTC(year, monthIdx, 0)).getUTCDate();
    const out: DayCell[] = [];
    // Leading days from previous month.
    for (let i = startOffset; i > 0; i--) {
      const d = new Date(first);
      d.setUTCDate(d.getUTCDate() - i);
      out.push({
        date: fmt(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate()),
        day: d.getUTCDate(),
        inMonth: false,
      });
    }
    for (let d = 1; d <= daysInMonth; d++) {
      out.push({ date: fmt(year, monthIdx - 1, d), day: d, inMonth: true });
    }
    // Trailing days to complete the last week.
    while (out.length % 7 !== 0) {
      const last = new Date(out[out.length - 1].date + "T00:00:00Z");
      last.setUTCDate(last.getUTCDate() + 1);
      out.push({
        date: fmt(last.getUTCFullYear(), last.getUTCMonth(), last.getUTCDate()),
        day: last.getUTCDate(),
        inMonth: false,
      });
    }
    return out;
  }, [year, monthIdx]);

  const eventsByDay = useMemo(() => {
    const m = new Map<string, Occurrence[]>();
    for (const e of events) {
      const arr = m.get(e.date) ?? [];
      arr.push(e);
      m.set(e.date, arr);
    }
    return m;
  }, [events]);

  const tasksByDay = useMemo(() => {
    const m = new Map<string, Task[]>();
    for (const t of tasksDue) {
      if (!t.due_date) continue;
      const arr = m.get(t.due_date) ?? [];
      arr.push(t);
      m.set(t.due_date, arr);
    }
    return m;
  }, [tasksDue]);

  const today = new Date().toISOString().slice(0, 10);
  const selectedEvents = eventsByDay.get(selected) ?? [];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">
          Calendar <span className="ml-2 font-mono text-xs text-muted-foreground">{month}</span>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-7 gap-px overflow-hidden rounded-md border bg-border">
          {WEEKDAYS.map((w) => (
            <div key={w} className="bg-muted/60 px-2 py-1.5 text-center text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
              {w}
            </div>
          ))}
          {cells.map((c) => {
            const evts = eventsByDay.get(c.date) ?? [];
            const tks = tasksByDay.get(c.date) ?? [];
            const isToday = c.date === today;
            const isSelected = c.date === selected;
            return (
              <button
                key={c.date}
                onClick={() => onSelect(c.date)}
                className={`min-h-[64px] bg-background p-1.5 text-left align-top transition-colors hover:bg-accent ${
                  isSelected ? "bg-accent" : ""
                } ${c.inMonth ? "" : "opacity-40"}`}
              >
                <span
                  className={`inline-flex h-5 w-5 items-center justify-center rounded-full text-xs ${
                    isToday ? "bg-foreground font-semibold text-background" : ""
                  }`}
                >
                  {c.day}
                </span>
                <div className="mt-0.5 space-y-0.5">
                  {evts.slice(0, 2).map((e) => (
                    <div key={e.event_id + e.date} className="truncate rounded-sm border px-1 py-px text-[10px] leading-tight">
                      {e.title}
                    </div>
                  ))}
                  {evts.length > 2 && (
                    <div className="text-[10px] text-muted-foreground">+{evts.length - 2} more</div>
                  )}
                  {tks.map((t) => (
                    <div key={t.id} className="truncate rounded-sm border border-dashed px-1 py-px text-[10px] leading-tight text-muted-foreground">
                      ☐ {t.title}
                    </div>
                  ))}
                </div>
              </button>
            );
          })}
        </div>

        {/* Agenda for the selected day */}
        <div className="mt-4">
          <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            Agenda · {selected}
          </h3>
          {selectedEvents.length === 0 ? (
            <p className="rounded-md border border-dashed p-3 text-xs text-muted-foreground">No events.</p>
          ) : (
            <ul className="space-y-1.5">
              {selectedEvents.map((e) => (
                <li key={e.event_id + e.date} className="flex items-center gap-2 rounded-md border p-2 text-sm">
                  <span className="font-mono text-xs tabular-nums text-muted-foreground">
                    {new Date(e.starts_at).toISOString().slice(11, 16)}
                  </span>
                  <span>{e.title}</span>
                  {e.location && <span className="text-xs text-muted-foreground">· {e.location}</span>}
                  {e.series && (
                    <span className="ml-auto rounded-full border px-1.5 py-px font-mono text-[10px] text-muted-foreground">
                      recurring
                    </span>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
