"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiSend } from "@/lib/api";

type Tab = "note" | "bookmark" | "reading" | "item";

export function QuickAdd({ onAdded }: { onAdded: () => void }) {
  const [tab, setTab] = useState<Tab>("note");
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [tags, setTags] = useState("");
  const [url, setUrl] = useState("");
  const [author, setAuthor] = useState("");
  const [itemType, setItemType] = useState("warranty");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState("");

  const parseTags = (): string[] =>
    tags
      .split(/[,\s]+/)
      .map((t) => t.replace(/^#/, "").trim())
      .filter(Boolean);

  const submit = async () => {
    setBusy(true);
    setMsg("");
    try {
      const t = parseTags();
      if (tab === "note") {
        await apiSend("/v1/notes", "POST", { title, body, tags: t });
        setMsg("Note saved.");
      } else if (tab === "bookmark") {
        await apiSend("/v1/bookmarks", "POST", { url, title, description: body, tags: t });
        setMsg("Bookmark saved (URL normalized + deduped).");
      } else if (tab === "reading") {
        await apiSend("/v1/reading", "POST", {
          title,
          author: author || null,
          url: url || null,
          tags: t,
        });
        setMsg("Added to reading list.");
      } else {
        await apiSend("/v1/items", "POST", { type: itemType, title, body, tags: t });
        setMsg(`Captured ${itemType}.`);
      }
      setTitle("");
      setBody("");
      setUrl("");
      setAuthor("");
      onAdded();
    } catch (e) {
      setMsg(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-3">
      <div className="flex gap-1">
        {(["note", "bookmark", "reading", "item"] as Tab[]).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`rounded-md border px-2.5 py-1 text-xs capitalize transition-colors ${
              tab === t ? "border-foreground bg-foreground text-background" : "text-muted-foreground hover:bg-accent"
            }`}
          >
            {t === "item" ? "capture" : `${t}`}
          </button>
        ))}
      </div>

      <div className="space-y-2">
        {tab === "bookmark" ? (
          <Input placeholder="https://…" value={url} onChange={(e) => setUrl(e.target.value)} className="h-8 text-sm" />
        ) : null}
        {tab === "reading" && (
          <Input placeholder="Author (optional)" value={author} onChange={(e) => setAuthor(e.target.value)} className="h-8 text-sm" />
        )}
        {(tab !== "bookmark" || true) && (
          <Input
            placeholder={tab === "bookmark" ? "Title (optional)" : "Title"}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="h-8 text-sm"
          />
        )}
        {tab === "item" && (
          <select
            value={itemType}
            onChange={(e) => setItemType(e.target.value)}
            className="h-8 w-full rounded-md border border-input bg-background px-2 text-xs"
          >
            {["warranty", "idea", "receipt", "contact", "misc"].map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        )}
        <textarea
          placeholder={tab === "note" ? "Body (markdown)…" : tab === "bookmark" ? "Description…" : "Notes…"}
          value={body}
          onChange={(e) => setBody(e.target.value)}
          rows={3}
          className="w-full rounded-md border border-input bg-background px-2 py-1.5 text-sm"
        />
        <Input
          placeholder="#tags comma or space"
          value={tags}
          onChange={(e) => setTags(e.target.value)}
          className="h-8 font-mono text-xs"
        />
        <div className="flex items-center justify-between">
          <span className="text-xs text-muted-foreground">{msg}</span>
          <Button size="sm" disabled={busy || (!title.trim() && !url.trim())} onClick={() => void submit()}>
            Save
          </Button>
        </div>
      </div>
    </div>
  );
}
