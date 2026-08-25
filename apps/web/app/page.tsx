import { ThemeToggle } from "@/components/theme-toggle";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import Link from "next/link";

const pillars = [
  {
    title: "Finance",
    desc: "Accounts, transactions, budgets, CSV import + dedupe, rules. Spending by month/category vs budget.",
    status: "LIVE",
    href: "/finance",
    live: true,
  },
  {
    title: "Planner",
    desc: "Tasks + habits + calendar events. Today / upcoming / streaks / recurrence.",
    status: "LIVE",
    href: "/planner",
    live: true,
  },
  {
    title: "Knowledge",
    desc: "Notes + bookmarks + reading list. FTS5, tags, links, global search.",
    status: "LIVE",
    href: "/knowledge",
    live: true,
  },
  {
    title: "Health",
    desc: "Meals + recipes + grocery, workouts + body metrics. Trends that matter.",
    status: "LIVE",
    href: "/health",
    live: true,
  },
] as const;

export default function Home() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="sticky top-0 z-10 border-b bg-background/80 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
          <div className="flex items-center gap-3">
            <div className="h-7 w-7 rounded-md border bg-foreground" aria-hidden />
            <span className="text-sm font-semibold tracking-tight">Personal OS</span>
            <Badge variant="secondary" className="ml-2 hidden sm:inline-flex">local-first</Badge>
            <Badge variant="outline" className="hidden sm:inline-flex">agent-native</Badge>
          </div>
          <div className="flex items-center gap-2">
            <span className="hidden text-xs text-muted-foreground sm:inline">Monochrome · Dark/Light</span>
            <ThemeToggle />
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-6 py-10">
        <div className="max-w-3xl space-y-4">
          <div className="inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs text-muted-foreground">
            <span className="h-2 w-2 rounded-full bg-foreground" /> Phase 1 — Foundation
          </div>
          <h1 className="text-4xl font-semibold tracking-tight sm:text-5xl">
            One file, one API, every part of life — for you and your agents.
          </h1>
          <p className="text-base leading-7 text-muted-foreground sm:text-lg">
            Local-first platform merging the apps you actually use. Four pillars + a universal capture
            core, monochrome-visual, fully writable via REST and MCP. Portfolio-grade, not a todo skeleton.
          </p>
          <div className="flex flex-wrap gap-3 pt-2">
            <a href="/openapi.json" target="_blank" rel="noreferrer" className={buttonVariants()}>
              OpenAPI →
            </a>
            <a
              href="/healthz"
              target="_blank"
              rel="noreferrer"
              className={buttonVariants({ variant: "outline" })}
            >
              /healthz
            </a>
            <a
              href="https://github.com/davinakmalyasha/PersonalOS"
              target="_blank"
              rel="noreferrer"
              className={buttonVariants({ variant: "ghost" })}
            >
              GitHub
            </a>
          </div>
        </div>

        <section className="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {pillars.map((p) => (
            <Card key={p.title} className="flex flex-col">
              <CardHeader>
                <div className="flex items-center justify-between gap-2">
                  <CardTitle className="text-base">{p.title}</CardTitle>
                  <Badge variant={p.live ? "default" : "secondary"} className="text-[10px]">
                    {p.status}
                  </Badge>
                </div>
                <CardDescription className="leading-6">{p.desc}</CardDescription>
              </CardHeader>
              <CardContent className="mt-auto pt-0">
                {p.live ? (
                  <Link href={p.href} className={buttonVariants({ variant: "outline", size: "sm" }) + " w-full"}>
                    Open {p.href} →
                  </Link>
                ) : (
                  <Button variant="outline" size="sm" className="w-full" disabled>
                    Opens {p.href} →
                  </Button>
                )}
              </CardContent>
            </Card>
          ))}
        </section>

        <section className="mt-8 grid gap-4 lg:grid-cols-3">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Universal Capture</CardTitle>
              <CardDescription>Any random data via save_item. Type + title + body + JSON. FTS.</CardDescription>
            </CardHeader>
            <CardContent>
              <code className="block rounded-md border bg-muted p-3 font-mono text-xs leading-5">
                save_item {`{type:"warranty", title:"Headphones", data:{`}<br />
                &nbsp;&nbsp;expires:&quot;2027-03-01&quot;{`}, tags:["gear"]}`}<br />
                search_items {`{q:"warranty headphones"}`}
              </code>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Agent Access</CardTitle>
              <CardDescription>One JSON block in any MCP client.</CardDescription>
            </CardHeader>
            <CardContent>
              <code className="block rounded-md border bg-muted p-3 font-mono text-xs leading-5">
                {`{`}
                <br />
                &nbsp;&nbsp;&quot;mcp&quot;: {`{`}
                <br />
                &nbsp;&nbsp;&nbsp;&nbsp;&quot;personal-os&quot;: {`{`}
                <br />
                &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&quot;command&quot;: &quot;node&quot;,
                <br />
                &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&quot;args&quot;: [&quot;apps/mcp/dist/index.js&quot;],
                <br />
                &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&quot;env&quot;: {`{`}
                <br />
                &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&quot;PERSONAL_OS_URL&quot;: &quot;http://localhost:8080&quot;
                <br />
                &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;{`}`}
                <br />
                &nbsp;&nbsp;&nbsp;&nbsp;{`}`}
                <br />
                &nbsp;&nbsp;{`}`}
                <br />
                {`}`}
              </code>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Visual, Monochrome</CardTitle>
              <CardDescription>No hue. Greys, borders, weight do the work. Charts in grey ramp.</CardDescription>
            </CardHeader>
            <CardContent className="flex h-[120px] items-end gap-1">
              {[18, 35, 55, 75, 88].map((v, i) => (
                <div
                  key={i}
                  className="flex-1 rounded-t-sm border"
                  style={{ height: `${v}%`, background: `hsl(var(--chart-${i + 1}))` }}
                />
              ))}
            </CardContent>
          </Card>
        </section>

        <p className="pt-8 text-center text-xs text-muted-foreground">
          Go owns the DB · Bearer token · SQLite WAL · goose migrations · sqlc · Recharts monochrome · docs/spec covers everything
        </p>
      </main>
    </div>
  );
}
