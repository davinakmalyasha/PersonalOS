"use client";

// Neutral top bar: page links + live dot + theme toggle. No branding — this
// component is meant to disappear inside a host app.

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { ThemeToggle } from "@/components/theme-toggle";
import { ActivityProvider, useActivityLastChange } from "@/lib/use-activity";

const NAV = [
  { href: "/", label: "Overview" },
  { href: "/finance", label: "Finance" },
  { href: "/planner", label: "Planner" },
  { href: "/knowledge", label: "Knowledge" },
  { href: "/health", label: "Health" },
];

function LiveDot() {
  const lastChangeAt = useActivityLastChange();
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const i = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(i);
  }, []);

  const recent = lastChangeAt != null && now - lastChangeAt < 2500;
  const ago = lastChangeAt
    ? (() => {
        const s = Math.max(0, Math.round((now - lastChangeAt) / 1000));
        return s < 60 ? `${s}s ago` : `${Math.round(s / 60)}m ago`;
      })()
    : "";

  return (
    <span
      className="flex items-center gap-1.5 text-[10px] text-muted-foreground"
      title={ago ? `agent activity ${ago}` : "waiting for activity"}
    >
      <span className="relative flex h-2 w-2">
        {recent && (
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-foreground opacity-60" />
        )}
        <span
          className={`relative inline-flex h-2 w-2 rounded-full ${
            recent ? "bg-foreground" : "bg-muted-foreground/40"
          }`}
        />
      </span>
      {ago || "live"}
    </span>
  );
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();

  return (
    <ActivityProvider>
      <div className="flex min-h-screen flex-col bg-background text-foreground">
        <header className="sticky top-0 z-20 border-b bg-background/85 backdrop-blur">
          <div className="mx-auto flex h-12 max-w-6xl items-center justify-between px-6">
            <nav className="flex items-center gap-1">
              {NAV.map((n) => {
                const active =
                  n.href === "/" ? pathname === "/" : pathname.startsWith(n.href);
                return (
                  <Link
                    key={n.href}
                    href={n.href}
                    className={`rounded-md px-2.5 py-1 text-sm transition-colors ${
                      active
                        ? "bg-accent font-medium text-foreground"
                        : "text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {n.label}
                  </Link>
                );
              })}
            </nav>
            <div className="flex items-center gap-3">
              <LiveDot />
              <ThemeToggle />
            </div>
          </div>
        </header>
        <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-6">{children}</main>
      </div>
    </ActivityProvider>
  );
}
