"use client";

import { useMemo } from "react";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { WeightPoint } from "@/lib/api";

export function WeightChart({ points }: { points: WeightPoint[] }) {
  const data = useMemo(
    () => points.map((p) => ({ date: p.date.slice(5), weight_kg: p.weight_kg })),
    [points],
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Weight trend</CardTitle>
      </CardHeader>
      <CardContent className="h-56">
        {data.length < 2 ? (
          <p className="flex h-full items-center justify-center text-sm text-muted-foreground">
            Log weight on at least two days to see a trend.
          </p>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={data} margin={{ left: -18, right: 8, top: 8 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" vertical={false} />
              <XAxis
                dataKey="date"
                tick={{ fill: "hsl(var(--muted-foreground))", fontSize: 10 }}
                axisLine={{ stroke: "hsl(var(--border))" }}
                tickLine={false}
              />
              <YAxis
                domain={["auto", "auto"]}
                tick={{ fill: "hsl(var(--muted-foreground))", fontSize: 10 }}
                axisLine={false}
                tickLine={false}
                tickFormatter={(v: number) => `${v}`}
              />
              <Tooltip
                contentStyle={{
                  background: "hsl(var(--popover))",
                  border: "1px solid hsl(var(--border))",
                  borderRadius: 8,
                  color: "hsl(var(--popover-foreground))",
                  fontSize: 12,
                }}
                formatter={(v) => [`${Number(v).toFixed(1)} kg`, "weight"]}
              />
              <Line
                type="monotone"
                dataKey="weight_kg"
                stroke="hsl(var(--chart-2))"
                strokeWidth={2}
                dot={{ r: 2.5, fill: "hsl(var(--chart-2))" }}
                activeDot={{ r: 4 }}
              />
            </LineChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  );
}
