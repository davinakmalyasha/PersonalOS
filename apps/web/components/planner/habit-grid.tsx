"use client";

import { useMemo } from "react";
import { Bar, BarChart, ResponsiveContainer, Tooltip, XAxis } from "recharts";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { Habit } from "@/lib/api";

const DAYS = 28;

function lastNDates(n: number): string[] {
  const out: string[] = [];
  const now = new Date();
  for (let i = n - 1; i >= 0; i--) {
    const d = new Date(now);
    d.setUTCDate(d.getUTCDate() - i);
    out.push(d.toISOString().slice(0, 10));
  }
  return out;
}

function HabitTile({ habit }: { habit: Habit }) {
  const dates = useMemo(() => new Set(habit.dates ?? []), [habit.dates]);
  const window = useMemo(() => lastNDates(DAYS), []);
  const doneCount = window.filter((d) => dates.has(d)).length;
  const pct = Math.round((doneCount / DAYS) * 100);

  return (
    <div className="rounded-md border p-3">
      <div className="flex items-center justify-between gap-2">
        <div className="min-w-0">
          <p className="truncate text-sm font-medium">{habit.name}</p>
          <p className="text-[11px] text-muted-foreground">
            {habit.cadence === "weekly" ? `${habit.target_per_week}×/week` : "daily"} · {doneCount}/{DAYS} days ({pct}%)
          </p>
        </div>
        <Badge variant="outline" className="shrink-0 font-mono text-[10px]">
          🔥 {habit.streaks.current} · best {habit.streaks.longest}
        </Badge>
      </div>

      {/* Heatmap: 4 weeks × 7 days */}
      <div className="mt-2 grid w-fit grid-flow-col grid-rows-7 gap-[3px]">
        {window.map((d) => {
          const on = dates.has(d);
          return (
            <div
              key={d}
              title={d}
              className={`h-3.5 w-3.5 rounded-[2px] border ${on ? "border-foreground bg-foreground" : "bg-transparent opacity-60"}`}
            />
          );
        })}
      </div>

      {habit.cadence === "weekly" && (
        <div className="mt-2 flex items-center gap-2">
          <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full bg-foreground"
              style={{ width: `${Math.min(100, (habit.streaks.week_done / habit.target_per_week) * 100)}%` }}
            />
          </div>
          <span className="font-mono text-[10px] tabular-nums text-muted-foreground">
            {habit.streaks.week_done}/{habit.target_per_week} this week
          </span>
        </div>
      )}
    </div>
  );
}

export function HabitGrid({ habits }: { habits: Habit[] }) {
  // Chart-first: completions per day across all habits (last 28 days).
  const chartData = useMemo(() => {
    const window = lastNDates(DAYS);
    return window.map((day) => {
      let count = 0;
      for (const h of habits) {
        if ((h.dates ?? []).includes(day)) count++;
      }
      return { day: day.slice(8), count }; // label = day-of-month
    });
  }, [habits]);

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle className="text-sm">Habits</CardTitle>
        <span className="text-xs text-muted-foreground">last {DAYS} days</span>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="h-24">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={chartData} margin={{ left: 0, right: 0 }}>
              <XAxis
                dataKey="day"
                tick={{ fill: "hsl(var(--muted-foreground))", fontSize: 9 }}
                interval={6}
                axisLine={{ stroke: "hsl(var(--border))" }}
                tickLine={false}
              />
              <Tooltip
                cursor={{ fill: "hsl(var(--accent))" }}
                contentStyle={{
                  background: "hsl(var(--popover))",
                  border: "1px solid hsl(var(--border))",
                  borderRadius: 8,
                  color: "hsl(var(--popover-foreground))",
                  fontSize: 12,
                }}
              />
              <Bar dataKey="count" fill="hsl(var(--chart-2))" radius={[2, 2, 0, 0]} barSize={6} />
            </BarChart>
          </ResponsiveContainer>
        </div>

        {habits.length === 0 ? (
          <p className="rounded-md border border-dashed p-3 text-sm text-muted-foreground">
            No active habits yet.
          </p>
        ) : (
          <div className="grid gap-3 md:grid-cols-2">
            {habits.map((h) => (
              <HabitTile key={h.id} habit={h} />
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
