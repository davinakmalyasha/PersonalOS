"use client";

import { useEffect, useState } from "react";
import { Droplets } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { apiSend, todayStr, type ExercisePR, type HealthSettings, type VolumeRow } from "@/lib/api";

// Monochrome SVG ring: progress = used/target.
export function Ring({
  label,
  value,
  target,
  unit,
}: {
  label: string;
  value: number | null | undefined;
  target: number | null | undefined;
  unit?: string;
}) {
  const v = value ?? 0;
  const t = target ?? 0;
  const pct = t > 0 ? Math.min(100, Math.round((v / t) * 100)) : 0;
  const R = 26;
  const C = 2 * Math.PI * R;
  return (
    <div className="flex flex-col items-center gap-1">
      <div className="relative h-16 w-16">
        <svg viewBox="0 0 64 64" className="h-16 w-16 -rotate-90">
          <circle cx="32" cy="32" r={R} fill="none" stroke="currentColor" strokeWidth="5" className="text-muted" />
          <circle
            cx="32"
            cy="32"
            r={R}
            fill="none"
            stroke="currentColor"
            strokeWidth="5"
            strokeLinecap="round"
            strokeDasharray={`${(pct / 100) * C} ${C}`}
            className="text-foreground transition-all duration-500"
          />
        </svg>
        <span className="absolute inset-0 flex items-center justify-center font-mono text-xs font-semibold tabular-nums">
          {t > 0 ? `${pct}%` : "—"}
        </span>
      </div>
      <p className="text-[10px] uppercase tracking-wide text-muted-foreground">{label}</p>
      <p className="font-mono text-[11px] tabular-nums text-muted-foreground">
        {Math.round(v).toLocaleString()}
        {t > 0 ? ` / ${t.toLocaleString()}${unit ?? ""}` : (unit ?? "")}
      </p>
    </div>
  );
}

export function MacroRings({ summary }: { summary: { macros?: { calories?: number | null; protein_g?: number | null; carbs_g?: number | null; fat_g?: number | null }; settings?: HealthSettings | null } | null }) {
  const m = summary?.macros;
  const s = summary?.settings;
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Today vs targets</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Ring label="Calories" value={m?.calories} target={s?.calorie_target} />
          <Ring label="Protein" value={m?.protein_g} target={s?.protein_target_g} unit="g" />
          <Ring label="Carbs" value={m?.carbs_g} target={s?.carbs_target_g} unit="g" />
          <Ring label="Fat" value={m?.fat_g} target={s?.fat_target_g} unit="g" />
        </div>
        {(s?.calorie_target == null) && (
          <p className="mt-3 text-center text-[10px] text-muted-foreground">
            Set targets via the agent (<code className="font-mono">manage_health_settings</code>) or PUT /v1/health/settings.
          </p>
        )}
      </CardContent>
    </Card>
  );
}

export function WaterButton({ onChanged }: { onChanged?: () => void }) {
  const [busy, setBusy] = useState(false);
  const drink = async (ml: number) => {
    setBusy(true);
    try {
      await apiSend("/v1/body-metrics/water", "POST", { ml });
      onChanged?.();
    } finally {
      setBusy(false);
    }
  };
  return (
    <Button variant="outline" size="sm" disabled={busy} onClick={() => void drink(250)}>
      <Droplets className="mr-1 h-3.5 w-3.5" /> +250 ml
    </Button>
  );
}

export function PRTable({ prs }: { prs: ExercisePR[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Personal records</CardTitle>
      </CardHeader>
      <CardContent>
        {prs.length === 0 ? (
          <p className="rounded-md border border-dashed p-3 text-center text-xs text-muted-foreground">
            Log weighted sets to surface PRs.
          </p>
        ) : (
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b text-left text-muted-foreground">
                <th className="pb-1 font-medium">Exercise</th>
                <th className="pb-1 text-right font-medium">Max kg</th>
                <th className="pb-1 text-right font-medium">@reps</th>
                <th className="pb-1 text-right font-medium">Date</th>
              </tr>
            </thead>
            <tbody>
              {prs.slice(0, 8).map((pr) => (
                <tr key={pr.exercise} className="border-b last:border-0">
                  <td className="py-1.5 pr-2">{pr.exercise}</td>
                  <td className="py-1.5 text-right font-mono tabular-nums">{pr.max_weight_kg}</td>
                  <td className="py-1.5 text-right font-mono tabular-nums text-muted-foreground">{pr.best_reps_at_max || "—"}</td>
                  <td className="py-1.5 text-right font-mono tabular-nums text-muted-foreground">{pr.last_date}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </CardContent>
    </Card>
  );
}

export function VolumeTable({ rows }: { rows: VolumeRow[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Weekly tonnage</CardTitle>
      </CardHeader>
      <CardContent>
        {rows.length === 0 ? (
          <p className="rounded-md border border-dashed p-3 text-center text-xs text-muted-foreground">
            No training logged this week.
          </p>
        ) : (
          <ul className="space-y-1">
            {rows.slice(0, 8).map((r) => {
              const max = rows[0].volume_kg || 1;
              return (
                <li key={r.exercise} className="flex items-center gap-2 text-xs">
                  <span className="w-32 truncate">{r.exercise}</span>
                  <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
                    <div className="h-full rounded-full bg-foreground" style={{ width: `${Math.round((r.volume_kg / max) * 100)}%` }} />
                  </div>
                  <span className="w-20 shrink-0 text-right font-mono tabular-nums text-muted-foreground">
                    {Math.round(r.volume_kg).toLocaleString()} kg
                  </span>
                </li>
              );
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

export function todayRFC3339(): string {
  return `${todayStr()}T${new Date().toISOString().slice(11, 19)}Z`;
}

// ---- Targets + goal weight editor (PUT merge semantics) ----

type EditableSettings = {
  calorie_target: number | null;
  protein_target_g: number | null;
  carbs_target_g: number | null;
  fat_target_g: number | null;
  water_target_ml: number | null;
  weekly_workout_target: number | null;
  goal_weight_kg: number | null;
};

export function HealthSettingsCard({ settings, onChanged }: { settings?: HealthSettings | null; onChanged?: () => void }) {
  const [form, setForm] = useState<Record<string, string>>({});
  useEffect(() => {
    if (!settings) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setForm({
      calorie_target: settings.calorie_target?.toString() ?? "",
      protein_target_g: settings.protein_target_g?.toString() ?? "",
      carbs_target_g: settings.carbs_target_g?.toString() ?? "",
      fat_target_g: settings.fat_target_g?.toString() ?? "",
      water_target_ml: settings.water_target_ml?.toString() ?? "",
      weekly_workout_target: settings.weekly_workout_target?.toString() ?? "",
      goal_weight_kg: settings.goal_weight_kg?.toString() ?? "",
    });
  }, [settings]);
  const [busy, setBusy] = useState(false);

  const fields: { key: keyof EditableSettings; label: string }[] = [
    { key: "calorie_target", label: "kcal/day" },
    { key: "protein_target_g", label: "protein g" },
    { key: "carbs_target_g", label: "carbs g" },
    { key: "fat_target_g", label: "fat g" },
    { key: "water_target_ml", label: "water ml" },
    { key: "weekly_workout_target", label: "workouts/wk" },
    { key: "goal_weight_kg", label: "goal kg" },
  ];

  const save = async () => {
    setBusy(true);
    try {
      const body: Record<string, unknown> = {};
      for (const f of fields) {
        const raw = form[f.key]?.trim();
        if (raw === "") continue; // untouched → keep stored value
        const n = parseFloat(raw!);
        if (!Number.isFinite(n)) continue;
        body[f.key] = n;
      }
      await apiSend("/v1/health/settings", "PATCH", body);
      onChanged?.();
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Targets & goals</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
          {fields.map((f) => (
            <label key={f.key} className="space-y-0.5">
              <span className="block text-[10px] uppercase tracking-wide text-muted-foreground">{f.label}</span>
              <input
                inputMode="decimal"
                value={form[f.key] ?? ""}
                onChange={(e) => setForm((s) => ({ ...s, [f.key]: e.target.value }))}
                className="h-8 w-full rounded-md border border-input bg-background px-2 font-mono text-xs tabular-nums"
              />
            </label>
          ))}
        </div>
        <div className="text-right">
          <Button size="sm" variant="secondary" disabled={busy} onClick={() => void save()}>
            Save targets
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
