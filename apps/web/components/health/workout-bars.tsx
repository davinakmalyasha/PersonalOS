"use client";

import { useMemo } from "react";
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis } from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { Workout } from "@/lib/api";

// Minutes trained per day over the trailing 14 days.
export function WorkoutBars({ workouts }: { workouts: Workout[] }) {
  const data = useMemo(() => {
    const days: { day: string; minutes: number }[] = [];
    const now = new Date();
    for (let i = 13; i >= 0; i--) {
      const d = new Date(now);
      d.setUTCDate(d.getUTCDate() - i);
      days.push({ day: d.toISOString().slice(5, 10), minutes: 0 });
    }
    const byDay = new Map(days.map((d) => [d.day, d]));
    for (const w of workouts) {
      const key = new Date(w.performed_at).toISOString().slice(5, 10);
      const slot = byDay.get(key);
      if (slot) slot.minutes += w.duration_minutes ?? 0;
    }
    return days;
  }, [workouts]);

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle className="text-sm">Training minutes</CardTitle>
        <span className="text-xs text-muted-foreground">last 14 days</span>
      </CardHeader>
      <CardContent className="h-44">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} margin={{ left: -22, right: 4, top: 4 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" vertical={false} />
            <XAxis
              dataKey="day"
              tick={{ fill: "hsl(var(--muted-foreground))", fontSize: 9 }}
              axisLine={{ stroke: "hsl(var(--border))" }}
              tickLine={false}
              interval={2}
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
              formatter={(v) => [`${v} min`, "trained"]}
            />
            <Bar dataKey="minutes" fill="hsl(var(--chart-2))" radius={[3, 3, 0, 0]} barSize={16} />
          </BarChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
