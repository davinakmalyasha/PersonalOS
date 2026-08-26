import { z } from "zod";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { PersonalOSClient } from "./api.js";

type ToolArgs = Record<string, unknown>;

function ok(data: unknown) {
  return { content: [{ type: "text" as const, text: JSON.stringify(data, null, 2) }] };
}

function fail(err: unknown) {
  const message = err instanceof Error ? err.message : String(err);
  return {
    isError: true as const,
    content: [{ type: "text" as const, text: message }],
  };
}

// Wraps a handler so thrown ApiErrors surface as readable tool results.
async function run(fn: () => Promise<unknown>) {
  try {
    return ok(await fn());
  } catch (err) {
    return fail(err);
  }
}

const str = () => z.string();
const optStr = () => z.string().optional().describe("optional");
const intOpt = () => z.number().int().optional();

export function registerTools(server: McpServer, api: PersonalOSClient): void {
  // ---------- Universal ----------

  server.tool(
    "save_item",
    "Save ANY personal data into universal capture (searchable immediately). " +
      "Use for warranties, receipts, ideas, contacts â€” anything without a dedicated tool.",
    {
      type: z.string().describe("open vocabulary slug, e.g. warranty|idea|receipt|contact|misc"),
      title: z.string().describe("short title (max 300 chars)"),
      body: z.string().optional().describe("longer text / markdown"),
      data: z.record(z.unknown()).optional().describe("structured JSON object, e.g. {expires:'2027-03-01'}"),
      tags: z.array(z.string()).optional(),
    },
    async (a: ToolArgs) =>
      run(() =>
        api.post("/v1/items", {
          type: a.type,
          title: a.title,
          body: a.body ?? "",
          data: a.data ? JSON.stringify(a.data) : "{}",
          tags: a.tags ?? [],
        }),
      ),
  );

  server.tool(
    "search_items",
    "Full-text search across ALL saved items (ranked). Empty q returns recent items.",
    {
      q: str().describe("free text; operators are ignored safely"),
      type: optStr(),
      tag: optStr(),
      limit: intOpt().describe("default 20, max 100"),
    },
    async (a: ToolArgs) => run(() => api.get("/v1/search", a)),
  );

  server.tool(
    "get_item",
    "Fetch one item by id (includes links and structured data).",
    { id: str() },
    async (a: ToolArgs) => run(() => api.get(`/v1/items/${a.id}`)),
  );

  server.tool(
    "update_item",
    "Patch an existing item's title/body/data/tags; also pin/archive it.",
    {
      id: str(),
      title: z.string().optional(),
      body: z.string().optional(),
      data: z.record(z.unknown()).optional().describe("replacement JSON object"),
      tags: z.array(z.string()).optional(),
      pinned: z.boolean().optional().describe("pinned rows sort first everywhere"),
      archived: z.boolean().optional().describe("archived rows leave lists/search until restored"),
    },
    async (a: ToolArgs) =>
      run(() =>
        api.patch(`/v1/items/${a.id}`, {
          ...(a.title !== undefined ? { title: a.title } : {}),
          ...(a.body !== undefined ? { body: a.body } : {}),
          ...(a.data !== undefined ? { data: JSON.stringify(a.data) } : {}),
          ...(a.tags !== undefined ? { tags: a.tags } : {}),
          ...(a.pinned !== undefined ? { pinned: a.pinned } : {}),
          ...(a.archived !== undefined ? { archived: a.archived } : {}),
        }),
      ),
  );

  server.tool(
    "link_items",
    "Create a directed link between two items (ids come from search results, never invent them).",
    { from_id: str(), to_id: str(), kind: optStr() },
    async (a: ToolArgs) =>
      run(() => api.post(`/v1/items/${a.from_id}/links`, { to_id: a.to_id, kind: a.kind ?? "related" })),
  );

  server.tool(
    "global_search",
    "Search v2 across EVERYTHING — items FTS plus typed scans of tasks, meals, workouts and transactions. Use when the user says 'find' without naming a pillar.",
    { q: str(), limit: intOpt() },
    async (a: ToolArgs) => run(() => api.get("/v1/search", a)),
  );

  // ---------- Finance ----------

  server.tool(
    "list_transactions",
    "List/filter transactions (paged).",
    {
      account_id: optStr(),
      category_id: optStr(),
      from: optStr().describe("YYYY-MM-DD"),
      to: optStr().describe("YYYY-MM-DD"),
      q: optStr(),
      page: intOpt(),
      page_size: intOpt(),
    },
    async (a: ToolArgs) => run(() => api.get("/v1/transactions", a)),
  );

  server.tool(
    "create_transaction",
    "Add a transaction manually. amount_minor is signed integer minor units (spend negative, income positive).",
    {
      account_id: str(),
      amount_minor: z.number().int(),
      date: str().describe("YYYY-MM-DD"),
      merchant: optStr(),
      raw_description: optStr(),
      category_id: optStr(),
      tags: z.array(z.string()).optional(),
      notes: optStr(),
    },
    async (a: ToolArgs) => run(() => api.post("/v1/transactions", a)),
  );

  server.tool(
    "import_transactions_csv",
    "Import bank CSV text for an account. Dedupes by (date, amount, description-hash); auto-categorizes via rules.",
    { account_id: str(), csv_text: str(), date_format: optStr().describe("Go layout override, e.g. 02/01/2006") },
    async (a: ToolArgs) => run(() => api.importCsv(String(a.account_id), String(a.csv_text), a.date_format as string | undefined)),
  );

  server.tool(
    "spending_summary",
    "Month totals + per-category spend + budget-vs-spent. THE tool for 'how much did I spendâ€¦'.",
    {
      month: str().describe("YYYY-MM"),
      group_by: z.enum(["month", "category"]).optional(),
    },
    async (a: ToolArgs) => {
      const summary = await api.get("/v1/finance/summary", { month: a.month });
      let spending: unknown;
      try {
        spending = await api.get("/v1/finance/spending", { group_by: (a.group_by as string) ?? "category" });
      } catch {
        spending = null;
      }
      return ok({ summary, spending });
    },
  );

  server.tool("list_categories", "Category tree.", {}, async () => run(() => api.get("/v1/categories")));

  server.tool(
    "manage_categories",
    "Manage categories. action=create needs name; action=delete needs id.",
    {
      action: z.enum(["create", "update", "delete"]),
      id: optStr(),
      name: optStr(),
      parent_id: optStr(),
      color: optStr(),
    },
    async (a: ToolArgs) => {
      switch (a.action) {
        case "create":
          return run(() => api.post("/v1/categories", { name: a.name, parent_id: a.parent_id ?? null, color: a.color ?? null }));
        case "update":
          return run(() => api.patch(`/v1/categories/${a.id}`, { name: a.name }));
        default:
          return run(() => api.del(`/v1/categories/${a.id}`));
      }
    },
  );

  server.tool(
    "manage_rules",
    "Manage auto-categorization rules. action=list|create|update|delete.",
    {
      action: z.enum(["list", "create", "update", "delete"]),
      id: optStr(),
      pattern: optStr(),
      category_id: optStr(),
      priority: z.number().int().optional(),
    },
    async (a: ToolArgs) => {
      switch (a.action) {
        case "list":
          return run(() => api.get("/v1/rules"));
        case "create":
          return run(() => api.post("/v1/rules", { pattern: a.pattern, category_id: a.category_id, priority: a.priority }));
        case "update":
          return run(() => api.patch(`/v1/rules/${a.id}`, { pattern: a.pattern, category_id: a.category_id, priority: a.priority }));
        default:
          return run(() => api.del(`/v1/rules/${a.id}`));
      }
    },
  );

  server.tool(
    "manage_budgets",
    "Upsert a monthly budget (category_id + month + amount_minor) or list budgets.",
    {
      category_id: optStr(),
      month: optStr().describe("YYYY-MM"),
      amount_minor: z.number().int().optional(),
    },
    async (a: ToolArgs) =>
      a.category_id && a.month
        ? run(() => api.post("/v1/budgets", { category_id: a.category_id, month: a.month, amount_minor: a.amount_minor }))
        : run(() => api.get("/v1/budgets")),
  );

  // ---------- Planner ----------

  server.tool(
    "list_tasks",
    "List/filter tasks. status='open' covers todo+doing; due_before finds overdue work.",
    {
      status: z.enum(["open", "todo", "doing", "done"]).optional(),
      priority: z.enum(["low", "med", "high"]).optional(),
      due: optStr().describe("YYYY-MM-DD exact"),
      due_before: optStr().describe("YYYY-MM-DD inclusive scan"),
      project: optStr(),
      tag: optStr(),
      q: optStr(),
      page: intOpt(),
      page_size: intOpt(),
    },
    async (a: ToolArgs) => run(() => api.get("/v1/tasks", a)),
  );

  server.tool(
    "create_task",
    "Quick-capture a task.",
    {
      title: str(),
      notes: optStr(),
      priority: z.enum(["low", "med", "high"]).optional(),
      due_date: optStr().describe("YYYY-MM-DD"),
      project: optStr(),
      tags: z.array(z.string()).optional(),
    },
    async (a: ToolArgs) => run(() => api.post("/v1/tasks", a)),
  );

  server.tool(
    "update_task",
    "Patch task status/priority/due/title/notes. status=done stamps completed_at.",
    {
      id: str(),
      status: z.enum(["todo", "doing", "done"]).optional(),
      priority: z.enum(["low", "med", "high"]).optional(),
      due_date: optStr().describe("empty clears"),
      title: z.string().optional(),
      notes: z.string().optional(),
    },
    async (a: ToolArgs) => run(() => api.patch(`/v1/tasks/${a.id}`, a)),
  );

  server.tool(
    "manage_habits",
    "action=list shows habits with streaks; action=create adds one (cadence daily|weekly, target_per_week 1..7); action=archive hides it.",
    {
      action: z.enum(["list", "create", "archive"]),
      id: optStr(),
      name: optStr(),
      description: optStr(),
      cadence: z.enum(["daily", "weekly"]).optional(),
      target_per_week: z.number().int().min(1).max(7).optional(),
      archived: z.boolean().optional(),
    },
    async (a: ToolArgs) => {
      switch (a.action) {
        case "list":
          return run(() => api.get("/v1/habits"));
        case "create":
          return run(() =>
            api.post("/v1/habits", {
              name: a.name,
              description: a.description ?? "",
              cadence: a.cadence ?? "daily",
              target_per_week: a.target_per_week,
            }),
          );
        default:
          return run(() => api.patch(`/v1/habits/${a.id}`, { archived: a.archived ?? true }));
      }
    },
  );

  server.tool(
    "check_habit",
    "Toggle today's (or a given date's) checkoff for a habit. Returns new done state + updated streaks.",
    { habit_id: str(), date: optStr().describe("YYYY-MM-DD, defaults to today UTC") },
    async (a: ToolArgs) =>
      run(() => api.post(`/v1/habits/${a.habit_id}/checkoffs`, { date: a.date ?? "" })),
  );

  server.tool(
    "habit_streak",
    "Habit detail: current/longest streak, week progress vs target, recent checkoff dates.",
    { habit_id: str() },
    async (a: ToolArgs) => run(() => api.get(`/v1/habits/${a.habit_id}`)),
  );

  server.tool(
    "create_event",
    "Create calendar event. recurrence_rule grammar: FREQ=DAILY|WEEKLY|MONTHLY;INTERVAL=n;COUNT=n|UNTIL=YYYYMMDD.",
    {
      title: str(),
      starts_at: str().describe("RFC3339, e.g. 2026-08-25T09:00:00Z"),
      ends_at: optStr().describe("RFC3339"),
      location: optStr(),
      recurrence_rule: optStr(),
      tags: z.array(z.string()).optional(),
    },
    async (a: ToolArgs) => run(() => api.post("/v1/events", a)),
  );

  server.tool(
    "planner_today",
    "Everything actionable today: overdue tasks, tasks due, habits (due/done state), events. First call for 'what's on today?'.",
    {},
    async () => run(() => api.get("/v1/planner/today")),
  );

  server.tool(
    "planner_upcoming",
    "Per-day agenda for the next N days (events + tasks due).",
    { days: intOpt().describe("default 7, max 60") },
    async (a: ToolArgs) => run(() => api.get("/v1/planner/upcoming", { days: a.days })),
  );

  // ---------- Knowledge ----------

  server.tool(
    "create_note",
    "Save a markdown note (searchable immediately via unified FTS).",
    { title: str(), body: str(), tags: z.array(z.string()).optional() },
    async (a: ToolArgs) => run(() => api.post("/v1/notes", a)),
  );

  server.tool(
    "search_knowledge",
    "Cross-type FTS over notes, bookmarks and reading list. Empty-ish q returns recent captures.",
    { q: str(), type: z.enum(["note", "bookmark", "reading"]).optional(), tag: optStr(), limit: intOpt() },
    async (a: ToolArgs) => run(() => api.get("/v1/knowledge/search", a)),
  );

  server.tool(
    "create_bookmark",
    "Save a URL (normalized + deduped by canonical form). Returns existing bookmark on duplicate.",
    { url: str(), title: optStr(), description: optStr(), tags: z.array(z.string()).optional() },
    async (a: ToolArgs) => run(() => api.post("/v1/bookmarks", a)),
  );

  server.tool(
    "create_reading",
    "Add to reading list (status to-read|reading|done).",
    {
      title: str(),
      author: optStr(),
      url: optStr(),
      status: z.enum(["to-read", "reading", "done"]).optional(),
      tags: z.array(z.string()).optional(),
    },
    async (a: ToolArgs) => run(() => api.post("/v1/reading", a)),
  );

  server.tool(
    "update_reading",
    "Update reading progress/rating/notes. status=done stamps finished_at; rating 1..5.",
    {
      id: str(),
      status: z.enum(["to-read", "reading", "done"]).optional(),
      rating: z.number().int().min(1).max(5).nullable().optional(),
      notes: z.string().optional(),
    },
    async (a: ToolArgs) => run(() => api.patch(`/v1/reading/${a.id}`, a)),
  );

  // ---------- Health ----------

  server.tool(
    "log_meal",
    "Log what you ate. eaten_at defaults to now; items is an array like [{name:'rice', qty:'150', unit:'g'}].",
    {
      title: str(),
      eaten_at: optStr().describe("RFC3339, default now UTC"),
      items: z.array(z.record(z.unknown())).optional(),
      calories: z.number().int().min(0).optional(),
      protein_g: z.number().min(0).optional(),
      carbs_g: z.number().min(0).optional(),
      fat_g: z.number().min(0).optional(),
      notes: optStr(),
      tags: z.array(z.string()).optional(),
    },
    async (a: ToolArgs) =>
      run(() =>
        api.post("/v1/meals", {
          ...a,
          eaten_at:
            (a.eaten_at as string | undefined) ?? new Date().toISOString().replace(/\.\d{3}Z$/, "Z"),
          items: a.items === undefined ? undefined : JSON.stringify(a.items),
        }),
      ),
  );

  server.tool(
    "log_workout",
    "Log training. exercises is an array like [{name:'squat', sets:3, reps:8, weight_kg:100}]. performed_at defaults to now.",
    {
      performed_at: optStr().describe("RFC3339, default now UTC"),
      title: optStr(),
      duration_minutes: z.number().int().min(0).optional(),
      exercises: z.array(z.record(z.unknown())).optional(),
      notes: optStr(),
      tags: z.array(z.string()).optional(),
    },
    async (a: ToolArgs) =>
      run(() =>
        api.post("/v1/workouts", {
          ...a,
          performed_at:
            (a.performed_at as string | undefined) ?? new Date().toISOString().replace(/\.\d{3}Z$/, "Z"),
          exercises: a.exercises === undefined ? undefined : JSON.stringify(a.exercises),
        }),
      ),
  );

  server.tool(
    "log_weight",
    "Record body metrics; same-day re-log REPLACES that day's row. At least one of weight_kg/body_fat_pct required.",
    {
      weight_kg: z.number().positive().optional(),
      body_fat_pct: z.number().positive().max(99.9).optional(),
      measured_at: optStr().describe("RFC3339, default now UTC"),
      notes: optStr(),
    },
    async (a: ToolArgs) =>
      run(() => {
        if (!a.weight_kg && !a.body_fat_pct) throw new Error("provide weight_kg and/or body_fat_pct");
        return api.post("/v1/body-metrics", {
          ...a,
          measured_at:
            (a.measured_at as string | undefined) ?? new Date().toISOString().replace(/\.\d{3}Z$/, "Z"),
        });
      }),
  );

  server.tool(
    "health_summary",
    "Window rollup: workout count/minutes, meal count/calories, weight firstâ†’latest change, grocery counts. Defaults to the last 30 days.",
    { from: optStr().describe("YYYY-MM-DD"), to: optStr().describe("YYYY-MM-DD") },
    async (a: ToolArgs) => {
      const to =
        (a.to as string | undefined) ?? new Date().toISOString().slice(0, 10);
      const from =
        (a.from as string | undefined) ??
        new Date(Date.now() - 29 * 86400000).toISOString().slice(0, 10);
      return run(() => api.get("/v1/health/summary", { from, to }));
    },
  );

  server.tool(
    "manage_grocery",
    "Grocery list. action=list|add|toggle|clear_checked|remove.",
    {
      action: z.enum(["list", "add", "toggle", "clear_checked", "remove"]),
      id: optStr(),
      name: optStr(),
      qty: optStr(),
      checked: z.boolean().optional(),
    },
    async (a: ToolArgs) => {
      switch (a.action) {
        case "add":
          return run(() => api.post("/v1/grocery", { name: a.name, qty: a.qty ?? "" }));
        case "toggle": {
          // Read current state then force the opposite (API PATCH is absolute).
          const current = (await api.get<{ items: Array<{ id: string; checked: boolean }> }>(
            "/v1/grocery",
          )).items.find((i) => i.id === a.id);
          if (!current) throw new Error(`grocery item ${a.id} not found`);
          return run(() => api.patch(`/v1/grocery/${a.id}`, { checked: !current.checked }));
        }
        case "clear_checked":
          return run(() => api.post("/v1/grocery/clear-checked"));
        case "remove":
          return run(() => api.del(`/v1/grocery/${a.id}`));
        default:
          return run(() => api.get("/v1/grocery"));
      }
    },
  );

  // ---------- Phase 10a: finance + planner intelligence ----------

  server.tool(
    "net_worth",
    "Portfolio net-worth over time: cumulative per-account balances derived from all transactions.",
    {},
    async () => run(() => api.get("/v1/finance/net-worth")),
  );

  server.tool(
    "upcoming_bills",
    "Detected subscriptions due within the next N days.",
    { days: intOpt().describe("default 7, max 90") },
    async (a: ToolArgs) => run(() => api.get("/v1/finance/bills", { days: a.days })),
  );

  server.tool(
    "manage_merchants",
    "Merchant aliases: rewrite noisy bank merchants to clean names on create/import. action=list|create|delete.",
    {
      action: z.enum(["list", "create", "delete"]),
      id: optStr(),
      pattern: optStr().describe("substring matched case-insensitively"),
      canonical: optStr().describe("clean name to store"),
    },
    async (a: ToolArgs) => {
      switch (a.action) {
        case "list":
          return run(() => api.get("/v1/merchant_aliases"));
        case "create":
          return run(() => api.post("/v1/merchant_aliases", { pattern: a.pattern, canonical: a.canonical }));
        default:
          return run(() => api.del(`/v1/merchant_aliases/${a.id}`));
      }
    },
  );

  server.tool(
    "set_event_exception",
    "Override ONE occurrence of a recurring event: cancel it or edit fields for that date only.",
    {
      event_id: str(),
      date: str().describe("YYYY-MM-DD occurrence"),
      action: z.enum(["cancel", "edit"]),
      title: z.string().optional(),
      starts_at: optStr().describe("RFC3339"),
      ends_at: optStr().describe("RFC3339"),
      location: optStr(),
    },
    async (a: ToolArgs) =>
      run(() =>
        api.post(`/v1/events/${a.event_id}/exceptions`, {
          date: a.date,
          action: a.action,
          title: a.title ?? null,
          starts_at: a.starts_at ?? null,
          ends_at: a.ends_at ?? null,
          location: a.location ?? null,
        }),
      ),
  );

  server.tool(
    "review_week",
    "Weekly review bundle: completed tasks, habit consistency, events held, spend vs budgets.",
    { date: optStr().describe("any date inside the week (YYYY-MM-DD), default current week") },
    async (a: ToolArgs) => run(() => api.get("/v1/planner/review", { date: a.date })),
  );

  // ---------- Phase 10b: memory & findability ----------

  server.tool(
    "activity_feed",
    "Changelog of everything that changed recently — 'what did my agent just do?'. Optionally filter by entity (task|note|transaction|…).",
    {
      limit: intOpt().describe("default 30, max 200"),
      entity: optStr(),
    },
    async (a: ToolArgs) => run(() => api.get("/v1/activity/feed", a)),
  );

  server.tool(
    "manage_saved_searches",
    "Named reusable queries. action=list|create|update|delete|run. Stored query is {q, type, tag, limit}.",
    {
      action: z.enum(["list", "create", "update", "delete", "run"]),
      id: optStr().describe("required for update/delete/run"),
      name: optStr(),
      q: optStr().describe("create/update: query text"),
      type: optStr(),
      tag: optStr(),
      limit: intOpt(),
    },
    async (a: ToolArgs) => {
      const query = {
        ...(a.q !== undefined ? { q: a.q } : {}),
        ...(a.type !== undefined ? { type: a.type } : {}),
        ...(a.tag !== undefined ? { tag: a.tag } : {}),
        ...(a.limit !== undefined ? { limit: a.limit } : {}),
      };
      switch (a.action) {
        case "create":
          return run(() => api.post("/v1/saved_searches", { name: a.name, query }));
        case "update":
          return run(() => api.patch(`/v1/saved_searches/${a.id}`, { name: a.name, query }));
        case "delete":
          return run(() => api.del(`/v1/saved_searches/${a.id}`));
        case "run":
          return run(() => api.post(`/v1/saved_searches/${a.id}/run`));
        default:
          return run(() => api.get("/v1/saved_searches"));
      }
    },
  );

  server.tool(
    "daily_note",
    "Today's scratch note. action=get|append — append adds one bullet line (creates the note on first touch).",
    {
      action: z.enum(["get", "append"]).optional().default("get"),
      text: optStr().describe("append: the line to add"),
      date: optStr().describe("YYYY-MM-DD, default today"),
    },
    async (a: ToolArgs) => {
      if (a.action === "append") {
        if (!a.text) throw new Error("text required for append");
        return run(() =>
          api.patch("/v1/knowledge/daily", { text: a.text, date: a.date ?? null }),
        );
      }
      return run(() => api.get("/v1/knowledge/daily", { date: a.date }));
    },
  );

  server.tool(
    "resurface_memory",
    "On-this-day resurfacing: notes, bookmarks, readings and items captured on the same month-day in earlier years.",
    {
      date: optStr().describe("YYYY-MM-DD, default today"),
      limit: intOpt().describe("default 10, max 50"),
    },
    async (a: ToolArgs) => run(() => api.get("/v1/knowledge/resurface", a)),
  );

  server.tool(
    "backlinks",
    "Everything linked to/from an item — the knowledge-graph panel for any capture.",
    { id: str() },
    async (a: ToolArgs) => run(() => api.get(`/v1/items/${a.id}/links`)),
  );

  server.tool(
    "export_data",
    "Full JSON dump of every table — portability and backups in one call.",
    {},
    async () => run(() => api.get("/v1/export")),
  );

  // ---------- Phase 10c: health macros + wiring ----------

  server.tool(
    "manage_health_settings",
    "Daily macro/water targets + weekly workout target. action=get|set. set merges: only provided fields change.",
    {
      action: z.enum(["get", "set"]).optional().default("get"),
      calorie_target: z.number().int().min(0).optional(),
      protein_target_g: z.number().int().min(0).optional(),
      carbs_target_g: z.number().int().min(0).optional(),
      fat_target_g: z.number().int().min(0).optional(),
      water_target_ml: z.number().int().min(0).optional(),
      weekly_workout_target: intOpt().describe("1..14 sessions per week"),
    },
    async (a: ToolArgs) => {
      if (a.action === "get") return run(() => api.get("/v1/health/settings"));
      const body = {
        calorie_target: a.calorie_target,
        protein_target_g: a.protein_target_g,
        carbs_target_g: a.carbs_target_g,
        fat_target_g: a.fat_target_g,
        water_target_ml: a.water_target_ml,
        weekly_workout_target: a.weekly_workout_target,
      };
      return run(() => api.patch("/v1/health/settings", body));
    },
  );

  server.tool(
    "workout_volume",
    "Weekly tonnage: volume (weight x reps) per exercise within a window, heaviest load first.",
    { from: optStr().describe("YYYY-MM-DD"), to: optStr().describe("YYYY-MM-DD") },
    async (a: ToolArgs) => {
      const to =
        (a.to as string | undefined) ?? new Date().toISOString().slice(0, 10);
      const from =
        (a.from as string | undefined) ??
        new Date(Date.now() - 6 * 86400000).toISOString().slice(0, 10);
      return run(() => api.get("/v1/health/volume", { from, to }));
    },
  );

  server.tool(
    "measurement_trends",
    "Free-form body measurements (chest/waist/…) as per-key time series over a window.",
    { from: optStr().describe("YYYY-MM-DD"), to: optStr().describe("YYYY-MM-DD") },
    async (a: ToolArgs) => run(() => api.get("/v1/body-metrics/trends", a)),
  );

  // ---------- Phase 12a: finance depth ----------

  server.tool(
    "manage_subscriptions",
    "Managed recurring charges. action=list|create|sync|set_status — sync upserts detected subscriptions; set_status moves one through active|muted|cancelled.",
    {
      action: z.enum(["list", "create", "sync", "set_status"]).optional().default("list"),
      id: optStr().describe("set_status: subscription id"),
      status: z.enum(["active", "muted", "cancelled"]).optional(),
      merchant: optStr(),
      amount_minor: z.number().int().optional().describe("negative = charge"),
      cadence: z.enum(["weekly", "monthly", "yearly"]).optional(),
      next_due: optStr().describe("YYYY-MM-DD"),
      status_filter: z.enum(["active", "muted", "cancelled"]).optional().describe("list filter"),
    },
    async (a: ToolArgs) => {
      switch (a.action) {
        case "create":
          return run(() =>
            api.post("/v1/subscriptions", {
              merchant: a.merchant,
              amount_minor: a.amount_minor,
              cadence: a.cadence ?? null,
              next_due: a.next_due ?? null,
            }),
          );
        case "sync":
          return run(() => api.post("/v1/finance/subscriptions/sync"));
        case "set_status":
          if (!a.id || !a.status) throw new Error("id + status required for set_status");
          return run(() => api.patch(`/v1/subscriptions/${a.id}`, { status: a.status }));
        default:
          return run(() => api.get("/v1/subscriptions", { status: a.status_filter }));
      }
    },
  );

  server.tool(
    "safe_to_spend",
    "PocketGuard-style number: income MTD − spend MTD − unspent budgets − subscriptions still due this month.",
    { month: optStr().describe("YYYY-MM, default current") },
    async (a: ToolArgs) => run(() => api.get("/v1/finance/safe-to-spend", a)),
  );

  server.tool(
    "cashflow_forecast",
    "Projected balance day-by-day from current balances, active subscriptions and trailing-90d average net flow.",
    {
      days: intOpt().describe("default 30, max 120"),
      alert_below: z.number().int().optional().describe("minor units; adds alert=true when projected dips below"),
    },
    async (a: ToolArgs) => run(() => api.get("/v1/finance/forecast", a)),
  );

  server.tool(
    "recategorize_history",
    "Backfill: re-run a categorization rule over ALL transactions (respects its pattern + amount window). Returns how many moved.",
    { rule_id: str() },
    async (a: ToolArgs) => run(() => api.post(`/v1/rules/${a.rule_id}/apply`)),
  );

// ---------- Phase 9: agentic depth ----------

  server.tool(
    "manage_goals",
    "Savings goals (kind=savings, target_minor + saved_minor + deadline) and the single daily-calorie goal (kind=calorie, target_minor = kcal). action=list|create|update|add|delete.",
    {
      action: z.enum(["list", "create", "update", "add", "delete"]),
      kind: z.enum(["savings", "calorie"]).optional(),
      id: optStr(),
      name: optStr(),
      target_minor: z.number().int().optional(),
      saved_minor: z.number().int().optional(),
      deadline: optStr().describe("YYYY-MM-DD"),
      amount_minor: z.number().int().optional().describe("add: delta applied to saved_minor"),
    },
    async (a: ToolArgs) => {
      switch (a.action) {
        case "list":
          return run(() => api.get("/v1/goals", { kind: a.kind }));
        case "create":
          return run(() =>
            api.post("/v1/goals", {
              kind: a.kind,
              name: a.name,
              target_minor: a.target_minor ?? null,
              deadline: a.deadline ?? null,
            }),
          );
        case "update":
          return run(() =>
            api.patch(`/v1/goals/${a.id}`, {
              name: a.name,
              target_minor: a.target_minor,
              saved_minor: a.saved_minor,
              deadline: a.deadline,
            }),
          );
        case "add":
          return run(() => api.post(`/v1/goals/${a.id}/add`, { amount_minor: a.amount_minor }));
        default:
          return run(() => api.del(`/v1/goals/${a.id}`));
      }
    },
  );

  server.tool(
    "find_recurring",
    "Detect subscriptions: same merchant + amount recurring ~monthly. Great for 'what am I subscribed to?'",
    {},
    async () => run(() => api.get("/v1/finance/recurring")),
  );

  server.tool(
    "log_water",
    "Add milliliters to today's water intake.",
    { ml: z.number().int().min(1).max(10000), day: optStr().describe("YYYY-MM-DD, default today") },
    async (a: ToolArgs) => run(() => api.post("/v1/body-metrics/water", a)),
  );

  server.tool(
    "exercise_prs",
    "Personal records: heaviest set per exercise (from workout logs).",
    { from: optStr().describe("YYYY-MM-DD"), to: optStr().describe("YYYY-MM-DD") },
    async (a: ToolArgs) => run(() => api.get("/v1/health/prs", a)),
  );

  server.tool(
    "upcoming_expiries",
    "Items with a date-like data field (expires/due/…) inside the next N days — warranties, renewals.",
    { days: intOpt().describe("default 30") },
    async (a: ToolArgs) => run(() => api.get("/v1/items/expiring", { days: a.days })),
  );
}
