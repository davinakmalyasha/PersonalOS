"use client";

import { useCallback, useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { apiGet, apiSend, type Task } from "@/lib/api";

export function TasksTable({ onChanged }: { onChanged: () => void }) {
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("open");
  const [priority, setPriority] = useState("");
  const [page, setPage] = useState(1);
  const [title, setTitle] = useState("");
  const [due, setDue] = useState("");
  const [data, setData] = useState<{ items: Task[]; total: number; page_size: number } | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const params = new URLSearchParams({
        status,
        page: String(page),
        page_size: "20",
      });
      if (q) params.set("q", q);
      if (priority) params.set("priority", priority);
      const res = await apiGet<{ items: Task[]; total: number; page_size: number }>(
        `/v1/tasks?${params.toString()}`,
      );
      setData(res);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [status, page, q, priority]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const quickAdd = async () => {
    if (!title.trim()) return;
    try {
      await apiSend("/v1/tasks", "POST", {
        title: title.trim(),
        ...(due ? { due_date: due } : {}),
      });
      setTitle("");
      setDue("");
      await load();
      onChanged();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  };

  const toggleDone = async (t: Task) => {
    await apiSend(`/v1/tasks/${t.id}`, "PATCH", { status: t.status === "done" ? "todo" : "done" });
    await load();
    onChanged();
  };

  const remove = async (id: string) => {
    await apiSend(`/v1/tasks/${id}`, "DELETE");
    await load();
    onChanged();
  };

  const totalPages = data ? Math.max(1, Math.ceil(data.total / (data.page_size || 20))) : 1;

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle className="text-sm">Tasks</CardTitle>
        <div className="flex gap-2">
          <Input
            placeholder="Search…"
            value={q}
            onChange={(e) => {
              setQ(e.target.value);
              setPage(1);
            }}
            onKeyDown={(e) => e.key === "Enter" && void load()}
            className="h-8 w-40"
          />
          <select
            value={status}
            onChange={(e) => {
              setStatus(e.target.value);
              setPage(1);
            }}
            className="h-8 rounded-md border border-input bg-background px-2 text-xs"
          >
            <option value="open">Open</option>
            <option value="todo">Todo</option>
            <option value="doing">Doing</option>
            <option value="done">Done</option>
          </select>
          <select
            value={priority}
            onChange={(e) => {
              setPriority(e.target.value);
              setPage(1);
            }}
            className="h-8 rounded-md border border-input bg-background px-2 text-xs"
          >
            <option value="">All priorities</option>
            <option value="high">High</option>
            <option value="med">Med</option>
            <option value="low">Low</option>
          </select>
        </div>
      </CardHeader>
      <CardContent>
        {/* Quick capture */}
        <div className="mb-3 flex gap-2">
          <Input
            placeholder="Add a task…"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && void quickAdd()}
            className="h-8 flex-1"
          />
          <input
            type="date"
            value={due}
            onChange={(e) => setDue(e.target.value)}
            className="h-8 rounded-md border border-input bg-background px-2 font-mono text-xs"
          />
          <Button size="sm" onClick={() => void quickAdd()}>
            Add
          </Button>
        </div>

        {error && <p className="text-sm text-destructive">{error}</p>}
        {!error && (
          <div className="overflow-x-auto rounded-md border">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-muted/60 text-left text-xs uppercase tracking-wide text-muted-foreground">
                  <th className="px-3 py-2 font-medium">Title</th>
                  <th className="px-3 py-2 font-medium">Priority</th>
                  <th className="px-3 py-2 font-medium">Due</th>
                  <th className="px-3 py-2 font-medium">Status</th>
                  <th className="px-3 py-2 text-right font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {(data?.items ?? []).map((t) => (
                  <tr key={t.id} className="border-t">
                    <td className={`px-3 py-2 ${t.status === "done" ? "text-muted-foreground line-through" : ""}`}>
                      {t.title}
                    </td>
                    <td className="px-3 py-2">
                      <Badge
                        variant={t.priority === "high" ? "default" : t.priority === "low" ? "outline" : "secondary"}
                        className="text-[10px] uppercase"
                      >
                        {t.priority}
                      </Badge>
                    </td>
                    <td className="px-3 py-2 font-mono text-xs tabular-nums">{t.due_date ?? "—"}</td>
                    <td className="px-3 py-2">
                      <Badge variant={t.status === "done" ? "secondary" : "outline"} className="text-[10px] uppercase">
                        {t.status}
                      </Badge>
                    </td>
                    <td className="px-3 py-2 text-right">
                      <div className="flex justify-end gap-1">
                        <Button size="sm" variant="outline" className="h-6 px-2 text-xs" onClick={() => void toggleDone(t)}>
                          {t.status === "done" ? "Reopen" : "Done"}
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-6 px-2 text-xs text-muted-foreground"
                          onClick={() => void remove(t.id)}
                        >
                          ✕
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
                {data?.items.length === 0 && (
                  <tr>
                    <td colSpan={5} className="px-3 py-6 text-center text-muted-foreground">
                      No tasks match.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        )}
        <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground">
          <span>{data ? `${data.total} total` : ""}</span>
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="outline"
              disabled={page <= 1}
              onClick={() => setPage((p) => Math.max(1, p - 1))}
            >
              Prev
            </Button>
            <span>
              {page} / {totalPages}
            </span>
            <Button
              size="sm"
              variant="outline"
              disabled={page >= totalPages}
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            >
              Next
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
