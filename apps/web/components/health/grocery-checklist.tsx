"use client";

import { useCallback, useEffect, useState } from "react";
import { Check } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { apiGet, apiSend, type GroceryItem } from "@/lib/api";

export function GroceryChecklist({ reloadKey, onChanged }: { reloadKey: number; onChanged: () => void }) {
  const [items, setItems] = useState<GroceryItem[]>([]);
  const [name, setName] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const res = await apiGet<{ items: GroceryItem[] }>("/v1/grocery");
      setItems(res.items ?? []);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load, reloadKey]);

  const add = async () => {
    if (!name.trim()) return;
    await apiSend("/v1/grocery", "POST", { name: name.trim() });
    setName("");
    await load();
    onChanged();
  };

  const toggle = async (item: GroceryItem) => {
    await apiSend(`/v1/grocery/${item.id}`, "PATCH", { checked: !item.checked });
    await load();
    onChanged();
  };

  const clearChecked = async () => {
    await apiSend("/v1/grocery/clear-checked", "POST", {});
    await load();
    onChanged();
  };

  const unchecked = items.filter((i) => !i.checked);
  const checked = items.filter((i) => i.checked);

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle className="text-sm">Grocery</CardTitle>
        {checked.length > 0 && (
          <Button size="sm" variant="outline" className="h-6 px-2 text-[10px]" onClick={() => void clearChecked()}>
            Clear checked ({checked.length})
          </Button>
        )}
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex gap-2">
          <Input
            placeholder="Add item…"
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && void add()}
            className="h-8 flex-1"
          />
          <Button size="sm" onClick={() => void add()}>
            Add
          </Button>
        </div>

        {error && <p className="text-sm text-destructive">{error}</p>}

        <ul className="space-y-1.5">
          {[...unchecked, ...checked].map((i) => (
            <li key={i.id} className="flex items-center gap-2 rounded-md border p-2">
              <Button
                size="icon"
                variant={i.checked ? "default" : "outline"}
                className="h-5 w-5 shrink-0 rounded-full"
                onClick={() => void toggle(i)}
                aria-label={`Toggle ${i.name}`}
              >
                {i.checked && <Check className="h-3 w-3" />}
              </Button>
              <span className={`flex-1 truncate text-sm ${i.checked ? "text-muted-foreground line-through" : ""}`}>
                {i.name}
              </span>
              {(i.qty || i.unit) && (
                <Badge variant="outline" className="font-mono text-[10px]">
                  {i.qty} {i.unit}
                </Badge>
              )}
            </li>
          ))}
          {items.length === 0 && (
            <li className="rounded-md border border-dashed p-3 text-center text-xs text-muted-foreground">
              List is empty.
            </li>
          )}
        </ul>
      </CardContent>
    </Card>
  );
}
