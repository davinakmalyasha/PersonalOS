"use client";

import { useCallback, useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { WeightChart } from "@/components/health/weight-chart";
import { WorkoutBars } from "@/components/health/workout-bars";
import { GroceryChecklist } from "@/components/health/grocery-checklist";
import { RecentLog } from "@/components/health/recent-log";
import { MacroRings, PRTable, VolumeTable, WaterButton } from "@/components/health/rings-prs";
import {
  apiGet,
  apiSend,
  todayStr,
  type ExercisePR,
  type HealthSummary,
  type Meal,
  type TrendPoint,
  type VolumeRow,
  type WeightPoint,
  type Workout,
} from "@/lib/api";

function isoDaysAgo(n: number): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() - n);
  return d.toISOString().slice(0, 10);
}

export default function HealthPage() {
  const [summary, setSummary] = useState<HealthSummary | null>(null);
  const [weights, setWeights] = useState<WeightPoint[]>([]);
  const [workouts, setWorkouts] = useState<Workout[]>([]);
  const [meals, setMeals] = useState<Meal[]>([]);
  const [prs, setPrs] = useState<ExercisePR[]>([]);
  const [volume, setVolume] = useState<VolumeRow[]>([]);
  const [trends, setTrends] = useState<Record<string, TrendPoint[]>>({});
  const [reloadKey, setReloadKey] = useState(0);
  const [weightInput, setWeightInput] = useState("");
  const [error, setError] = useState("");

  const reload = useCallback(async () => {
    try {
      const to = todayStr();
      const from14 = isoDaysAgo(13);
      const from90 = isoDaysAgo(89);
      const [s, ws, wks, ms, pr, vol, tr] = await Promise.all([
        apiGet<HealthSummary>(`/v1/health/summary?from=${from14}&to=${to}`),
        apiGet<{ points: WeightPoint[] }>(`/v1/health/weight-series?from=${from90}&to=${to}`),
        apiGet<{ items: Workout[] }>(`/v1/workouts?from=${from14}&to=${to}&page_size=100`),
        apiGet<{ items: Meal[] }>(`/v1/meals?from=${isoDaysAgo(6)}&to=${to}&page_size=50`),
        apiGet<{ items: ExercisePR[] }>("/v1/health/prs"),
        apiGet<{ items: VolumeRow[] }>(`/v1/health/volume?from=${isoDaysAgo(6)}&to=${to}`),
        apiGet<{ trends: Record<string, TrendPoint[]> }>(`/v1/body-metrics/trends?from=${from90}&to=${to}`),
      ]);
      setSummary(s);
      setWeights(ws.points ?? []);
      setWorkouts(wks.items ?? []);
      setMeals(ms.items ?? []);
      setPrs(pr.items ?? []);
      setVolume(vol.items ?? []);
      setTrends(tr.trends ?? {});
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
    setReloadKey((k) => k + 1);
  }, []);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void reload();
  }, [reload]);

  const logWeight = async () => {
    const kg = parseFloat(weightInput.replace(",", "."));
    if (!Number.isFinite(kg) || kg <= 0) return;
    await apiSend("/v1/body-metrics", "POST", {
      measured_at: `${todayStr()}T${new Date().toISOString().slice(11, 19)}Z`,
      weight_kg: kg,
    });
    setWeightInput("");
    await reload();
  };

  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="sticky top-0 z-10 border-b bg-background/80 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-3">
          <span className="text-sm font-semibold tracking-tight">Health</span>
          <div className="flex items-center gap-2">
            <input
              type="number"
              step="0.1"
              placeholder="kg…"
              value={weightInput}
              onChange={(e) => setWeightInput(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && void logWeight()}
              className="h-8 w-24 rounded-md border border-input bg-background px-2 font-mono text-xs"
            />
            <Button size="sm" variant="secondary" onClick={() => void logWeight()}>
              Log weight
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

        {/* Summary cards */}
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {[
            {
              label: "Workouts · 14d",
              value: summary ? String(summary.workouts.count) : "—",
              sub: summary ? `${summary.workouts.total_minutes} min total` : "",
            },
            {
              label: "Meals logged",
              value: summary ? String(summary.meals.count) : "—",
              sub:
                summary?.meals.calories_total != null
                  ? `${summary.meals.calories_total.toLocaleString()} kcal`
                  : "no calories tracked",
            },
            {
              label: "Latest weight",
              value: summary?.weight.latest_kg != null ? `${summary.weight.latest_kg.toFixed(1)} kg` : "—",
              sub:
                summary?.weight.change_kg != null
                  ? `${summary.weight.change_kg > 0 ? "+" : ""}${summary.weight.change_kg.toFixed(1)} kg in window`
                  : "",
            },
            {
              label: "Grocery",
              value: summary ? `${summary.grocery.checked}/${summary.grocery.total}` : "—",
              sub: "checked / total items",
            },
          ].map((c) => (
            <Card key={c.label}>
              <CardHeader className="pb-0">
                <CardTitle className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  {c.label}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="font-mono text-2xl font-semibold tabular-nums">{c.value}</p>
                <p className="mt-0.5 text-[11px] text-muted-foreground">{c.sub}</p>
              </CardContent>
            </Card>
          ))}
        </div>

        {/* Macro rings + water */}
        <div className="grid gap-4 lg:grid-cols-3">
          <div className="lg:col-span-2">
            <MacroRings summary={summary} />
          </div>
          <Card>
            <CardContent className="flex h-full flex-col items-center justify-center gap-2 py-4">
              <p className="text-[10px] uppercase tracking-wide text-muted-foreground">Water today</p>
              <p className="font-mono text-2xl font-semibold tabular-nums">
                {summary?.water_today_ml != null ? `${(summary.water_today_ml / 1000).toFixed(2)}L` : "0.00L"}
              </p>
              {summary?.settings?.water_target_ml != null && (
                <p className="text-[10px] text-muted-foreground">
                  target {(summary.settings.water_target_ml / 1000).toFixed(1)}L
                </p>
              )}
              <WaterButton onChanged={() => void reload()} />
            </CardContent>
          </Card>
        </div>

        <div className="grid gap-4 lg:grid-cols-5">
          <div className="lg:col-span-3">
            <WeightChart points={weights} />
          </div>
          <div className="lg:col-span-2">
            <WorkoutBars workouts={workouts} />
          </div>
        </div>

        <div className="grid gap-4 lg:grid-cols-2">
          <PRTable prs={prs} />
          <VolumeTable rows={volume} />
        </div>

        {Object.keys(trends).length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Measurement trends · 90d</CardTitle>
            </CardHeader>
            <CardContent>
              <ul className="space-y-1 text-xs">
                {Object.entries(trends).map(([key, pts]) => {
                  const first = pts[0]?.value ?? 0;
                  const last = pts[pts.length - 1]?.value ?? 0;
                  const delta = last - first;
                  return (
                    <li key={key} className="flex items-center justify-between rounded-md border p-2">
                      <span>{key.replace(/_/g, " ")}</span>
                      <span className="font-mono tabular-nums text-muted-foreground">
                        {pts.length} pts · latest {last} ({delta >= 0 ? "+" : ""}{Number(delta.toFixed(1))})
                      </span>
                    </li>
                  );
                })}
              </ul>
            </CardContent>
          </Card>
        )}

        <div className="grid gap-4 lg:grid-cols-5">
          <div className="lg:col-span-3">
            <RecentLog meals={meals} workouts={workouts} />
          </div>
          <div className="lg:col-span-2">
            <GroceryChecklist reloadKey={reloadKey} onChanged={() => void reload()} />
          </div>
        </div>
      </main>
    </div>
  );
}
