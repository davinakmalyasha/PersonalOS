"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { Meal, Workout } from "@/lib/api";

function timeOf(iso: string): string {
  return new Date(iso).toISOString().slice(11, 16);
}

export function RecentLog({ meals, workouts }: { meals: Meal[]; workouts: Workout[] }) {
  // Unified timeline, newest first.
  const entries = [
    ...meals.map((m) => ({
      key: `meal-${m.id}`,
      kind: "Meal" as const,
      at: m.eaten_at,
      title: m.title,
      detail:
        (m.calories != null ? `${m.calories} kcal` : "") +
        (m.items.length ? ` · ${m.items.length} items` : ""),
    })),
    ...workouts.map((w) => ({
      key: `workout-${w.id}`,
      kind: "Workout" as const,
      at: w.performed_at,
      title: w.title ?? "Training session",
      detail:
        (w.duration_minutes != null ? `${w.duration_minutes} min` : "") +
        (w.exercises.length ? ` · ${w.exercises.length} exercises` : ""),
    })),
  ].sort((a, b) => (a.at < b.at ? 1 : -1)).slice(0, 12);

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle className="text-sm">Recent log</CardTitle>
        <span className="text-xs text-muted-foreground">meals + workouts</span>
      </CardHeader>
      <CardContent>
        {entries.length === 0 ? (
          <p className="rounded-md border border-dashed p-4 text-center text-sm text-muted-foreground">
            Nothing logged yet.
          </p>
        ) : (
          <ul className="space-y-1.5">
            {entries.map((e) => (
              <li key={e.key} className="flex items-center gap-3 rounded-md border p-2.5">
                <span className="font-mono text-xs tabular-nums text-muted-foreground">{timeOf(e.at)}</span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm">{e.title}</p>
                  {e.detail && <p className="truncate text-[11px] text-muted-foreground">{e.detail}</p>}
                </div>
                <span className="rounded-full border px-1.5 py-px font-mono text-[9px] uppercase text-muted-foreground">
                  {e.kind}
                </span>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
