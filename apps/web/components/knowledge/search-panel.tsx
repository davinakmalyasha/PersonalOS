"use client";

import { useCallback, useEffect, useState } from "react";
import { ExternalLink, Link2, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiGet, apiSend, type KnowledgeItem, type LinkPair, type TagCount } from "@/lib/api";

const TYPE_LABELS: Record<string, string> = {
  note: "Note",
  bookmark: "Bookmark",
  reading: "Reading",
};

export function SearchPanel({ reloadKey }: { reloadKey: number }) {
  const [q, setQ] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [tagFilter, setTagFilter] = useState("");
  const [results, setResults] = useState<KnowledgeItem[]>([]);
  const [tags, setTags] = useState<TagCount[]>([]);
  const [expandedLinks, setExpandedLinks] = useState<string | null>(null);
  const [links, setLinks] = useState<{ outgoing: LinkPair[]; incoming: LinkPair[] } | null>(null);
  const [linkTarget, setLinkTarget] = useState("");
  const [error, setError] = useState("");

  const runSearch = useCallback(async () => {
    try {
      const params = new URLSearchParams();
      if (q) params.set("q", q);
      else params.set("q", ""); // empty → recent captures
      if (tagFilter) params.set("tag", tagFilter);
      params.set("limit", "50");
      let path = `/v1/knowledge/search?${params.toString()}`;
      if (typeFilter === "item") {
        // non-pillar capture types live only in the universal list
        const res = await apiGet<{ items: KnowledgeItem[] }>(`/v1/items?q=${encodeURIComponent(q)}&page_size=50`);
        setResults(res.items ?? []);
        setError("");
        return;
      }
      if (typeFilter) path += `&type=${typeFilter}`;
      const res = await apiGet<{ items: KnowledgeItem[] }>(path);
      setResults(res.items ?? []);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [q, tagFilter, typeFilter]);

  const loadTags = useCallback(async () => {
    try {
      const res = await apiGet<{ items: TagCount[] }>("/v1/knowledge/tags?limit=30");
      setTags(res.items ?? []);
    } catch {
      /* non-fatal */
    }
  }, []);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void runSearch();
    void loadTags();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [reloadKey, typeFilter, tagFilter]);

  const openLinks = async (itemId: string) => {
    if (expandedLinks === itemId) {
      setExpandedLinks(null);
      return;
    }
    try {
      const res = await apiGet<{ outgoing: LinkPair[]; incoming: LinkPair[] }>(`/v1/items/${itemId}/links`);
      setLinks(res);
      setExpandedLinks(itemId);
      setLinkTarget("");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  };

  const addLink = async (fromID: string) => {
    if (!linkTarget.trim()) return;
    await apiSend(`/v1/items/${fromID}/links`, "POST", { to_id: linkTarget.trim(), kind: "related" });
    await openLinks(fromID);
  };

  const removeLink = async (fromID: string, toID: string, kind: string) => {
    await apiSend(`/v1/items/${fromID}/links/${toID}/${kind}`, "DELETE");
    await openLinks(fromID);
  };

  return (
    <div className="space-y-4">
      {/* Search-first */}
      <div className="flex flex-wrap gap-2">
        <Input
          placeholder="Search knowledge… (notes, bookmarks, reading)"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && void runSearch()}
          className="h-10 min-w-[240px] flex-1 text-base"
          autoFocus
        />
        <Button size="lg" className="h-10" onClick={() => void runSearch()}>
          Search
        </Button>
      </div>

      {/* Type + tag filters */}
      <div className="flex flex-wrap items-center gap-1.5">
        {["", "note", "bookmark", "reading"].map((t) => (
          <button
            key={t || "all"}
            onClick={() => setTypeFilter(t)}
            className={`rounded-full border px-3 py-1 text-xs transition-colors ${
              typeFilter === t ? "border-foreground bg-foreground text-background" : "hover:bg-accent"
            }`}
          >
            {t === "" ? "All" : TYPE_LABELS[t]}
          </button>
        ))}
        <span className="mx-1 h-4 w-px bg-border" />
        {tags.slice(0, 12).map((t) => (
          <button
            key={t.tag}
            onClick={() => setTagFilter(tagFilter === t.tag ? "" : t.tag)}
            className={`rounded-full border px-2.5 py-1 font-mono text-xs transition-colors ${
              tagFilter === t.tag ? "border-foreground bg-foreground text-background" : "text-muted-foreground hover:bg-accent"
            }`}
          >
            #{t.tag} · {t.count}
          </button>
        ))}
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {/* Results */}
      <ul className="space-y-2">
        {!error &&
          results.map((r) => (
            <li key={r.id} className="rounded-md border p-3">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <Badge variant={r.type === "note" ? "default" : "secondary"} className="text-[10px] uppercase">
                      {TYPE_LABELS[r.type] ?? r.type}
                    </Badge>
                    <p className="truncate text-sm font-medium">{r.title}</p>
                  </div>
                  {r.body && <p className="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">{r.body}</p>}
                  <div className="mt-1.5 flex flex-wrap gap-1">
                    {r.tags.map((tg) => (
                      <span key={tg} className="font-mono text-[10px] text-muted-foreground">#{tg}</span>
                    ))}
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-1">
                  {(() => {
                    try {
                      const d = JSON.parse(r.data) as { url?: string };
                      if (d.url)
                        return (
                          <a href={d.url} target="_blank" rel="noreferrer" className="text-muted-foreground hover:text-foreground">
                            <ExternalLink className="h-3.5 w-3.5" />
                          </a>
                        );
                    } catch {
                      /* ignore */
                    }
                    return null;
                  })()}
                  <Button size="icon" variant="ghost" className="h-6 w-6" onClick={() => void openLinks(r.id)} aria-label="Links">
                    <Link2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>

              {/* Link affordance */}
              {expandedLinks === r.id && links && (
                <div className="mt-3 space-y-2 rounded-md border bg-muted/40 p-2.5">
                  {[...links.outgoing.map((l) => ({ dir: "out" as const, l })), ...links.incoming.map((l) => ({ dir: "in" as const, l }))].map(
                    ({ dir, l }) => (
                      <div key={dir + (l.to_id ?? "") + (l.from_id ?? "") + l.kind} className="flex items-center gap-2 text-xs">
                        <Badge variant="outline" className="font-mono text-[9px]">{l.kind}</Badge>
                        <span>{dir === "out" ? "→" : "←"}</span>
                        <span className="truncate">{dir === "out" ? l.to_title : l.from_title}</span>
                        <button
                          className="ml-auto text-muted-foreground hover:text-destructive"
                          onClick={() =>
                            dir === "out" && l.to_id
                              ? removeLink(r.id, l.to_id, l.kind)
                              : Promise.resolve()
                          }
                          aria-label="Remove link"
                        >
                          <Trash2 className="h-3 w-3" />
                        </button>
                      </div>
                    ),
                  )}
                  <div className="flex gap-1.5 pt-1">
                    <Input
                      placeholder="Paste an item id to link…"
                      value={linkTarget}
                      onChange={(e) => setLinkTarget(e.target.value)}
                      onKeyDown={(e) => e.key === "Enter" && void addLink(r.id)}
                      className="h-7 flex-1 font-mono text-[11px]"
                    />
                    <Button size="sm" variant="outline" className="h-7" onClick={() => void addLink(r.id)}>
                      Link
                    </Button>
                  </div>
                </div>
              )}
            </li>
          ))}
        {!error && results.length === 0 && (
          <li className="rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground">
            Nothing here yet — capture something on the right.
          </li>
        )}
      </ul>
    </div>
  );
}
