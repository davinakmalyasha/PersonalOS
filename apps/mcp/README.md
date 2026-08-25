# MCP — Personal OS server

`personal-os` MCP server (TypeScript, `@modelcontextprotocol/sdk`, stdio) — a thin wrapper over the Go API. 33 tools covering all pillars + universal capture. Tool catalog: `docs/mcp-tools.md`.

## Build & smoke

```bash
cd apps/mcp
npm install
npm run build          # compiles TS -> dist/
node scripts/mcp-smoke.mjs   # full JSON-RPC round-trip against a running API (set PERSONAL_OS_TOKEN)
```

## Run

Requires `PERSONAL_OS_URL` (default `http://localhost:8080`) and `PERSONAL_OS_TOKEN` (must match the API's `API_TOKEN`). The server logs startup diagnostics to stderr: is the API reachable? is the token set?

## Wiring

### opencode — `opencode.json`
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
Same shape in `claude_desktop_config.json` under `mcpServers.personal-os`.

## Troubleshooting

| Symptom | Fix |
|---|---|
| stderr: `API not reachable at …` | start the Go API (`go run -tags sqlite_fts5 ./services/api/cmd/api`) |
| tool errors with `unauthorized` | set `PERSONAL_OS_TOKEN` to the same value as the API's `API_TOKEN` |
| tools return empty results | seed data via the dashboard or ask the agent to create some |
