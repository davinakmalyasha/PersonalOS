"use client";

// Live activity: polls /v1/activity every 4s (paused when hidden), exposes
// per-pillar change versions so tiles re-fetch only when their data moved.

import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { API_BASE } from "@/lib/api";

export type ActivityData = {
  pillars: Record<string, string>;
  latest: string;
};

type ActivityState = {
  versions: Record<string, string>;
  lastChangeAt: number | null;
  connected: boolean;
};

const ActivityCtx = createContext<ActivityState>({
  versions: {},
  lastChangeAt: null,
  connected: false,
});

export function usePillarVersion(pillar: string): string | undefined {
  const { versions } = useContext(ActivityCtx);
  return versions[pillar];
}

export function useActivityLastChange(): number | null {
  const { lastChangeAt } = useContext(ActivityCtx);
  return lastChangeAt;
}

export function useActivityConnected(): boolean {
  const { connected } = useContext(ActivityCtx);
  return connected;
}

export function ActivityProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<ActivityState>({
    versions: {},
    lastChangeAt: null,
    connected: false,
  });
  const baseline = useRef<Record<string, string> | null>(null);

  useEffect(() => {
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;

    const poll = async () => {
      if (!document.hidden) {
        try {
          const res = await fetch(`${API_BASE}/v1/activity`, { cache: "no-store" });
          if (res.ok) {
            const data = (await res.json()) as ActivityData;
            const pillars = data.pillars ?? {};
            const first = baseline.current === null;
            let changed = false;
            if (!first) {
              for (const [k, v] of Object.entries(pillars)) {
                if (baseline.current![k] !== undefined && baseline.current![k] !== v) {
                  changed = true;
                  window.dispatchEvent(
                    new CustomEvent("personal-os:changed", { detail: k }),
                  );
                }
              }
            }
            baseline.current = pillars;
            setState((s) => ({
              versions: pillars,
              lastChangeAt: !first && changed ? Date.now() : s.lastChangeAt,
              connected: true,
            }));
          }
        } catch {
          if (!stopped) setState((s) => ({ ...s, connected: false }));
        }
      }
      if (!stopped) timer = setTimeout(poll, 4000);
    };

    void poll();
    return () => {
      stopped = true;
      clearTimeout(timer);
    };
  }, []);

  const value = useMemo(() => state, [state]);
  return <ActivityCtx.Provider value={value}>{children}</ActivityCtx.Provider>;
}
