export const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export async function apiGet<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, { cache: "no-store" });
  if (!res.ok) throw new Error(`GET ${path} failed: ${res.status}`);
  return res.json() as Promise<T>;
}

export async function apiSend<T>(
  path: string,
  method: "POST" | "PATCH" | "PUT" | "DELETE",
  body?: unknown,
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(text || `${method} ${path} failed: ${res.status}`);
  }
  return (res.status === 204 ? ({} as T) : (res.json() as Promise<T>)) as Promise<T>;
}

export type Account = {
  id: string;
  name: string;
  type: string;
  currency: string;
  created_at: string;
  balance_minor: number;
};

export type Transaction = {
  id: string;
  account_id: string;
  amount_minor: number;
  currency: string;
  date: string;
  merchant: string;
  raw_description: string;
  category_id: string | null;
  category_name: string | null;
  notes: string;
  created_at: string;
};

export type CategoryNode = {
  id: string;
  name: string;
  parent_id: string | null;
  color: string | null;
  created_at: string;
  children: CategoryNode[];
};

export type Rule = {
  id: string;
  pattern: string;
  category_id: string;
  priority: number;
  created_at: string;
};

export type Budget = {
  id: string;
  category_id: string;
  category?: string;
  month: string;
  amount_minor: number;
};

export type ImportResult = {
  imported: number;
  skipped: number;
  skipped_invalid: number;
  auto_categorized: number;
  errors?: { line: number; message: string }[];
};

export type MonthSummary = {
  month: string;
  income_minor: number;
  outcome_minor: number;
  net_minor: number;
  by_category: { category_id?: string; name: string; spent_minor: number }[];
  budgets: {
    category_id: string;
    category_name: string;
    budget_minor: number;
    spent_minor: number;
    over: boolean;
  }[];
};

export function currentMonth(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

// ---- Planner ----

export type Task = {
  id: string;
  title: string;
  notes: string;
  status: "todo" | "doing" | "done";
  priority: "low" | "med" | "high";
  due_date: string | null;
  due_time: string | null;
  project: string | null;
  recurrence_rule: string | null;
  series_id: string | null;
  parent_id: string | null;
  blocked_by: string | null;
  estimate_minutes: number | null;
  tags: string[];
  created_at: string;
  updated_at: string;
  completed_at: string | null;
};

export type HabitStreaks = {
  current: number;
  longest: number;
  done_today: boolean;
  week_done: number;
  target_per_week: number;
};

export type Habit = {
  id: string;
  name: string;
  description: string;
  cadence: "daily" | "weekly";
  target_per_week: number;
  weekdays: string; // Mon-first mask e.g. "1110100"
  paused_until: string | null;
  color: string | null;
  created_at: string;
  archived_at: string | null;
  dates?: string[];
  streaks: HabitStreaks;
};

export type Occurrence = {
  event_id: string;
  title: string;
  description: string;
  location: string;
  tags: string[];
  date: string;
  starts_at: string;
  ends_at?: string;
  series: boolean;
};

export type TodayBundle = {
  date: string;
  overdue: Task[];
  due_today: Task[];
  habits: (Habit & { due_today: boolean; done_today: boolean })[];
  events: Occurrence[];
};

export function todayStr(): string {
  return new Date().toISOString().slice(0, 10);
}

// ---- Knowledge + universal capture ----

export type KnowledgeItem = {
  id: string;
  type: string;
  title: string;
  body: string;
  data: string; // JSON object string
  tags: string[];
  source: "manual" | "api" | "mcp" | "import" | "promotion";
  source_item_id: string | null;
  pinned: boolean;
  archived: boolean;
  created_at: string;
  updated_at: string;
};

// ---- Memory (phase 10b) ----

export type Change = {
  entity: string;
  entity_id: string;
  action: "create" | "update" | "delete" | "complete";
  title: string;
  at: string;
};

export type UpcomingBill = {
  merchant: string;
  amount_minor: number;
  next_guess: string; // YYYY-MM-DD
  days_left: number;
  occurrences: number;
};

export type Subscription = {
  id: string;
  merchant: string;
  amount_minor: number;
  cadence: string;
  next_due: string | null;
  status: "active" | "muted" | "cancelled";
  occurrences: number;
};

export type SafeToSpend = {
  month: string;
  income_mtd_minor: number;
  spend_mtd_minor: number;
  budget_left_minor: number;
  bills_ahead_minor: number;
  safe_to_spend_minor: number;
};

export type ForecastPoint = { date: string; projected_minor: number };

export type Forecast = {
  days: number;
  start_minor: number;
  avg_daily_net_minor: number;
  lowest?: { date: string; projected_minor: number } | null;
  points: ForecastPoint[];
};

export type TagCount = { tag: string; count: number };

export type ReadingEntry = {
  id: string;
  title: string;
  author: string | null;
  url: string | null;
  status: "to-read" | "reading" | "done";
  rating: number | null;
  notes: string;
  tags: string[];
  created_at: string;
  updated_at: string;
  finished_at: string | null;
};

export type LinkPair = {
  from_id?: string;
  to_id?: string;
  kind: string;
  to_type?: string;
  to_title?: string;
  from_type?: string;
  from_title?: string;
  created_at: string;
};

// ---- Health ----

export type Meal = {
  id: string;
  eaten_at: string;
  title: string;
  notes: string;
  items: { name?: string; qty?: string; unit?: string }[];
  calories: number | null;
  protein_g: number | null;
  carbs_g: number | null;
  fat_g: number | null;
  tags: string[];
  created_at: string;
  updated_at: string;
};

export type Workout = {
  id: string;
  performed_at: string;
  title: string | null;
  notes: string;
  duration_minutes: number | null;
  exercises: { name?: string; sets?: number; reps?: number; weight_kg?: number }[];
  tags: string[];
  created_at: string;
  updated_at: string;
};

export type GroceryItem = {
  id: string;
  name: string;
  qty: string;
  unit: string | null;
  checked: boolean;
  recipe_id: string | null;
  created_at: string;
  updated_at: string;
};

export type WeightPoint = { date: string; weight_kg: number };

export type HealthSettings = {
  calorie_target: number | null;
  protein_target_g: number | null;
  carbs_target_g: number | null;
  fat_target_g: number | null;
  water_target_ml: number | null;
  weekly_workout_target: number | null;
  goal_weight_kg: number | null;
  updated_at: string;
};

export type VolumeRow = {
  exercise: string;
  sets: number;
  reps_total: number;
  volume_kg: number;
  max_weight_kg: number;
};

export type TrendPoint = { date: string; value: number };

export type ExercisePR = {
  exercise: string;
  max_weight_kg: number;
  best_reps_at_max: number;
  last_date: string;
};

export type HealthSummary = {
  from: string;
  to: string;
  workouts: { count: number; total_minutes: number };
  meals: { count: number; calories_total: number | null };
  weight: {
    latest_kg: number | null;
    first_kg: number | null;
    change_kg: number | null;
    measured_on: string | null;
  };
  grocery: { total: number; checked: number };
  macros: {
    calories?: number | null;
    protein_g?: number | null;
    carbs_g?: number | null;
    fat_g?: number | null;
  };
  settings?: HealthSettings | null;
  calorie_goal?: number | null;
  calories_today?: number | null;
  water_today_ml?: number | null;
};

export function formatMinor(minor: number, currency = "IDR"): string {
  const major = minor / 100;
  try {
    return new Intl.NumberFormat("id-ID", {
      style: "currency",
      currency,
      maximumFractionDigits: 0,
    }).format(major);
  } catch {
    return major.toLocaleString();
  }
}
