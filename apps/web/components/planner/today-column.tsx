"use client";

import { Check, Circle } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { Task } from "@/lib/api";

function TaskRow({
  task,
  onToggle,
  done = false,
}: {
  task: Task;
  onToggle: (t: Task) => void;
  done?: boolean;
}) {
  return (
    <li className={`flex items-start gap-2 rounded-md border p-2 ${task.parent_id ? "ml-5" : ""}`}>
      <Button
        size="icon"
        variant={done ? "default" : "outline"}
        className="h-5 w-5 shrink-0 rounded-full"
        onClick={() => onToggle(task)}
        aria-label={done ? "Reopen" : "Complete"}
      >
        {done ? <Check className="h-3 w-3" /> : <Circle className="h-2.5 w-2.5 opacity-40" />}
      </Button>
      <div className="min-w-0 flex-1">
        <p className={`truncate text-sm ${done ? "text-muted-foreground line-through" : ""}`}>{task.title}</p>
        <div className="mt-1 flex flex-wrap items-center gap-1">
          {task.due_date && (
            <span className="font-mono text-[10px] tabular-nums text-muted-foreground">
              {task.due_date}
              {task.due_time ? ` ${task.due_time}` : ""}
            </span>
          )}
          {task.priority !== "med" && (
            <Badge variant={task.priority === "high" ? "default" : "secondary"} className="text-[9px] uppercase">
              {task.priority}
            </Badge>
          )}
          {task.recurrence_rule && (
            <Badge variant="outline" className="font-mono text-[9px]">
              ↻ recurring
            </Badge>
          )}
          {task.blocked_by && (
            <Badge variant="outline" className="font-mono text-[9px] uppercase text-destructive/80">
              blocked
            </Badge>
          )}
          {task.parent_id && (
            <Badge variant="secondary" className="text-[9px] uppercase">
              subtask
            </Badge>
          )}
          {task.estimate_minutes != null && task.estimate_minutes > 0 && (
            <span className="font-mono text-[10px] tabular-nums text-muted-foreground">~{task.estimate_minutes}m</span>
          )}
        </div>
      </div>
    </li>
  );
}

export function TodayColumn({
  overdue,
  dueToday,
  habits,
  events,
  onToggleTask,
  onToggleHabit,
}: {
  overdue: Task[];
  dueToday: Task[];
  habits: {
    id: string;
    name: string;
    due_today: boolean;
    done_today: boolean;
    streaks: { current: number };
  }[];
  events: { event_id: string; title: string; starts_at: string; series: boolean }[];
  onToggleTask: (t: Task) => void;
  onToggleHabit: (habitId: string) => void;
}) {
  const todayEvents = events.filter((e) => e.starts_at.slice(0, 10) === new Date().toISOString().slice(0, 10));
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Today</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <section>
          <h3 className="mb-1.5 text-[11px] font-semibold uppercase tracking-wide text-destructive/80">
            Overdue · {overdue.length}
          </h3>
          {overdue.length === 0 ? (
            <p className="rounded-md border border-dashed p-2 text-xs text-muted-foreground">Nothing overdue.</p>
          ) : (
            <ul className="space-y-1.5">
              {overdue.map((t) => (
                <TaskRow key={t.id} task={t} onToggle={onToggleTask} />
              ))}
            </ul>
          )}
        </section>

        <section>
          <h3 className="mb-1.5 flex items-center justify-between text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
            <span>Due today · {dueToday.length}</span>
            <span className="font-mono text-[10px] normal-case tabular-nums">
              day load ~
              {dueToday.reduce((sum, t) => sum + (t.estimate_minutes ?? 0), 0) +
                overdue.reduce((sum, t) => sum + (t.estimate_minutes ?? 0), 0)}{" "}
              min
            </span>
          </h3>
          {dueToday.length === 0 ? (
            <p className="rounded-md border border-dashed p-2 text-xs text-muted-foreground">Clear day.</p>
          ) : (
            <ul className="space-y-1.5">
              {dueToday.map((t) => (
                <TaskRow key={t.id} task={t} onToggle={onToggleTask} />
              ))}
            </ul>
          )}
        </section>

        <section>
          <h3 className="mb-1.5 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Habits</h3>
          {habits.length === 0 ? (
            <p className="rounded-md border border-dashed p-2 text-xs text-muted-foreground">No active habits.</p>
          ) : (
            <ul className="space-y-1.5">
              {habits.map((h) => (
                <li key={h.id} className="flex items-center gap-2 rounded-md border p-2">
                  <Button
                    size="icon"
                    variant={h.done_today ? "default" : "outline"}
                    className="h-5 w-5 shrink-0 rounded-full"
                    onClick={() => onToggleHabit(h.id)}
                    aria-label={`Check off ${h.name}`}
                  >
                    {h.done_today ? <Check className="h-3 w-3" /> : <Circle className="h-2.5 w-2.5 opacity-40" />}
                  </Button>
                  <span className="flex-1 truncate text-sm">{h.name}</span>
                  <Badge variant="outline" className="font-mono text-[10px]">
                    🔥 {h.streaks.current}
                  </Badge>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section>
          <h3 className="mb-1.5 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Events</h3>
          {todayEvents.length === 0 ? (
            <p className="rounded-md border border-dashed p-2 text-xs text-muted-foreground">No events today.</p>
          ) : (
            <ul className="space-y-1.5">
              {todayEvents.map((e) => (
                <li key={e.event_id + e.starts_at} className="flex items-center gap-2 rounded-md border p-2 text-sm">
                  <span className="font-mono text-xs tabular-nums text-muted-foreground">
                    {new Date(e.starts_at).toISOString().slice(11, 16)}
                  </span>
                  <span className="truncate">{e.title}</span>
                </li>
              ))}
            </ul>
          )}
        </section>
      </CardContent>
    </Card>
  );
}
