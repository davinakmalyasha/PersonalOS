# MCP Tools — `personal-os` Server

One TypeScript MCP server (`apps/mcp`) wraps the Go API. No direct DB access. Transport: **stdio** (primary). Auth: `PERSONAL_OS_URL` + `PERSONAL_OS_TOKEN` from env.

**Rule verified in acceptance:** from a real MCP client, asking *"how much did I spend this month?"* returns correct data from the Go API.

## Config (client side)

### opencode
`opencode.json` (or `opencode.jsonc`):
```json
{
  "mcp": {
    "personal-os": {
      "command": "node",
      "args": ["D:/MyProject/PersonalOsMCP/apps/mcp/dist/index.js"],
      "env": {
        "PERSONAL_OS_URL": "http://localhost:8080",
        "PERSONAL_OS_TOKEN": "your-api-token"
      }
    }
  }
}
```

### Claude Desktop / Claude Code
Same shape in `claude_desktop_config.json` (key `mcpServers.personal-os`).

Build step: `npm --workspace apps/mcp run build` (compiles TS → `dist/`).

---

## Tool catalog

### Universal (covers every category, including future ones)

| Tool | Description | Input | Calls |
|---|---|---|---|
| `save_item` | create any personal data | `{type, title, body?, data?, tags?}` | `POST /v1/items` |
| `search_items` | FTS + filters across all items | `{q?, type?, tag?, limit?}` | `GET /v1/search` |
| `get_item` | fetch one item with links | `{id}` | `GET /v1/items/{id}` |
| `update_item` | patch any item | `{id, title?, body?, data?, tags?}` | `PATCH /v1/items/{id}` |
| `link_items` | relate two items | `{from_id, to_id, kind?}` | `POST /v1/items/{id}/links` |
| `global_search` | alias to `search_items` + knowledge alias | `{q, limit?}` | `GET /v1/search` |

With these six tools an agent can already manage *any* personal data you will ever invent.

### Finance power tools (typed, for real analytics)

| Tool | Description | Input | Calls |
|---|---|---|---|
| `list_transactions` | filtered transactions | `{account_id?, category_id?, from?, to?, q?, page?, page_size?}` | `GET /v1/transactions` |
| `create_transaction` | add transaction | `{account_id, amount, currency?, date, merchant?, raw_description, category_id?, notes?}` | `POST /v1/transactions` |
| `import_transactions_csv` | import CSV path (server reads file or base64) | `{account_id, csv_text}` | `POST /v1/transactions/import` |
| `spending_summary` | spend by month/category + vs budget | `{month, group_by?}` | `GET /v1/finance/summary` + `spending` |
| `list_categories` | category tree | `{}` | `GET /v1/categories` |
| `manage_categories` | create/update/delete category | `{action, id?, name?, parent_id?}` | `/v1/categories` |
| `manage_rules` | CRUD for categorization rules | `{action, id?, pattern?, category_id?, priority?}` | `/v1/rules` |
| `manage_budgets` | upsert budget | `{category_id, month, amount}` | `POST /v1/budgets` |

### Planner power tools

| Tool | Description | Input | Calls |
|---|---|---|---|
| `list_tasks` | task search | `{status?, priority?, due?, project?, q?, page?}` | `GET /v1/tasks` |
| `create_task` | quick capture | `{title, notes?, priority?, due_date?, project?, tags?}` | `POST /v1/tasks` |
| `update_task` | patch status/priority/due | `{id, status?, priority?, due_date?, title?, notes?}` | `PATCH /v1/tasks/{id}` |
| `manage_habits` | create/list habits | `{action, id?, name?, description?}` | `/v1/habits` |
| `check_habit` | toggle checkoff | `{habit_id, date}` | `POST /v1/habits/{id}/checkoffs` |
| `habit_streak` | get streak + recent checkoffs | `{habit_id}` | `GET /v1/habits/{id}` |
| `create_event` | create calendar event | `{title, starts_at, ends_at?, location?, recurrence_rule?, tags?}` | `POST /v1/events` |
| `planner_today` | today's agenda bundle | `{date?}` | `GET /v1/planner/today` |
| `planner_upcoming` | next N days | `{days?}` | `GET /v1/planner/upcoming` |

### Knowledge power tools

| Tool | Description | Input | Calls |
|---|---|---|---|
| `create_note` | create note | `{title, body, tags?}` | `POST /v1/notes` |
| `search_knowledge` | FTS notes/bookmarks/reading | `{q, type?, tag?, limit?}` | `GET /v1/knowledge/search` |
| `create_bookmark` | save URL | `{url, title?, description?, tags?}` | `POST /v1/bookmarks` |
| `create_reading` | add to reading list | `{title, author?, url?, status?, tags?}` | `POST /v1/reading` |
| `update_reading` | update progress/rating | `{id, status?, rating?, notes?}` | `PATCH /v1/reading/{id}` |

### Health power tools

| Tool | Description | Input | Calls |
|---|---|---|---|
| `log_meal` | log a meal | `{eaten_at?, title, items?, calories?, notes?, tags?}` | `POST /v1/meals` |
| `log_workout` | log workout | `{performed_at?, title?, duration_minutes?, exercises?, notes?}` | `POST /v1/workouts` |
| `log_weight` | upsert body metric | `{measured_at?, weight_kg?, body_fat_pct?, notes?}` | `POST /v1/body-metrics` |
| `health_summary` | rollup `from→to` | `{from?, to?}` | `GET /v1/health/summary` |
| `manage_grocery` | list/add/toggle grocery | `{action, id?, name?, qty?, checked?}` | `/v1/grocery` |

### Stretch servers (documented but out of v1 scope)
- `scout` — `fetch_url → markdown` (readability extraction)
- `files` — path-restricted local file search

## Tool design rules

- Every tool that mutates has an `id` return and is retry-safe where a natural key exists.
- Input field names are **snake_case, identical to the REST API** (`account_id`, `due_date`, …) so schemas map 1:1 with zero translation bugs (ADR-0019).
- Errors are returned as `isError` results carrying the API's message + forwarded `details[]`; auth failures include a diagnostic hint.
- No tool ever invents IDs — IDs come from the API.
- JSON-array fields (`items`, `exercises`, `ingredients`) accept **real arrays** from the agent; the server stringifies for the wire.

## Example flows

**Finance Q&A**
> User: "how much did I spend this month on food?"
Agent: calls `spending_summary({month:"2026-08"})` → filters `category: Food` → answers with number + chart link.

**Quick capture anywhere**
> User: "remember warranty for headphones expires 2027-03-01"
Agent: `save_item({type:"warranty", title:"Headphones warranty", body:"Expires 2027-03-01", data:{expires:"2027-03-01"}, tags:["gear"]})`

**Today's plan**
> User: "what's on today?"
Agent: `planner_today({})` → returns tasks + habits + events in one call.

## Security

- Server reads `PERSONAL_OS_TOKEN` once at boot; every request adds `Authorization: Bearer $TOKEN`. Missing token → 401 from API when `API_TOKEN` is set.
- Never logs the token. On auth failure returns a short diagnostic (did you set `PERSONAL_OS_TOKEN`? is the API up at `PERSONAL_OS_URL`?).
