"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { MonthCalendar } from "@/components/planner/month-calendar";
import { TodayColumn } from "@/components/planner/today-column";
import { TasksTable } from "@/components/planner/tasks-table";
import { HabitGrid } from "@/components/planner/habit-grid";
import {
  apiGet,
  apiSend,
  currentMonth,
  todayStr,
  type Habit,
  type Occurrence,
  type Task,
  type TodayBundle,
} from "@/lib/api";

export default function PlannerPage() {
  const [month, setMonth] = useState(currentMonth());
  const [selected, setSelected] = useState(todayStr());
  const [today, setToday] = useState<TodayBundle | null>(null);
  const [habits, setHabits] = useState<Habit[]>([]);
  const [openTasks, setOpenTasks] = useState<Task[]>([]);
  const [events, setEvents] = useState<Occurrence[]>([]);
  const [error, setError] = useState("");

  const reload = useCallback(async () => {
    try {
      const monthStart = `${month}-01`;
      const [y, m] = month.split("-").map(Number);
      const lastDay = new Date(Date.UTC(y, m, 0)).getUTCDate();
      const monthEnd = `${month}-${String(lastDay).padStart(2, "0")}`;

      const [t, h, tasks, evs] = await Promise.all([
        apiGet<TodayBundle>("/v1/planner/today"),
        apiGet<{ items: Habit[] }>("/v1/habits"),
        apiGet<{ items: Task[] }>("/v1/tasks?status=open&due_before=2099-12-31&page_size=100"),
        apiGet<{ items: Occurrence[] }>(`/v1/events?from=${monthStart}&to=${monthEnd}`),
      ]);
      setToday(t);
      setHabits(h.items ?? []);
      setOpenTasks(tasks.items ?? []);
      setEvents(evs.items ?? []);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [month]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void reload();
  }, [reload]);

  const toggleTask = async (t: Task) => {
    await apiSend(`/v1/tasks/${t.id}`, "PATCH", { status: t.status === "done" ? "todo" : "done" });
    await reload();
  };

  const toggleHabit = async (habitId: string) => {
    await apiSend(`/v1/habits/${habitId}/checkoffs`, "POST", { date: todayStr() });
    await reload();
  };

  const monthShift = (delta: number) => {
    const [y, m] = month.split("-").map(Number);
    const d = new Date(Date.UTC(y, m - 1 + delta, 1));
    setMonth(`${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}`);
  };

  const todayEvents = useMemo(() => {
    const seen = new Set<string>();
    const merged: Occurrence[] = [];
    for (const e of [...(today?.events ?? []), ...events]) {
      const key = `${e.event_id}|${e.date}`;
      if (!seen.has(key)) {
        seen.add(key);
        merged.push(e);
      }
    }
    return merged;
  }, [today, events]);

  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="sticky top-0 z-10 border-b bg-background/80 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-3">
          <span className="text-sm font-semibold tracking-tight">Planner</span>
          <div className="flex items-center gap-2">
            <a
              href={`${process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"}/v1/planner/calendar.ics`}
              className="rounded-md border px-2 py-1 font-mono text-[10px] text-muted-foreground hover:text-foreground"
              title="Subscribe from any calendar app"
            >
              .ics feed
            </a>
            <Button size="sm" variant="ghost" onClick={() => monthShift(-1)}>
              ←
            </Button>
            <input
              type="month"
              value={month}
              onChange={(e) => setMonth(e.target.value || currentMonth())}
              className="h-8 rounded-md border border-input bg-background px-2 font-mono text-xs"
            />
            <Button size="sm" variant="ghost" onClick={() => monthShift(1)}>
              →
            </Button>
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

        <div className="grid gap-4 lg:grid-cols-5">
          <div className="lg:col-span-2">
            <TodayColumn
              overdue={today?.overdue ?? []}
              dueToday={today?.due_today ?? []}
              habits={today?.habits ?? []}
              events={todayEvents}
              onToggleTask={toggleTask}
              onToggleHabit={toggleHabit}
            />
          </div>
          <div className="lg:col-span-3">
            <MonthCalendar
              month={month}
              selected={selected}
              onSelect={setSelected}
              events={events}
              tasksDue={openTasks}
            />
          </div>
        </div>

        <HabitGrid habits={habits} />

        <TasksTable onChanged={() => void reload()} />
      </main>
    </div>
  );
}
