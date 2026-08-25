# MCP — Personal OS server

`personal-os` MCP server (TypeScript, `@modelcontextprotocol/sdk`) — thin wrapper over the Go API.

## Build

```bash
npm --workspace apps/mcp run build   # compiles TS → dist/
```

## Run

Requires `PERSONAL_OS_URL` (default `http://localhost:8080`) and `PERSONAL_OS_TOKEN` (must match API `API_TOKEN`).

Wiring docs: see `docs/mcp-tools.md`.

## Status

Phase 1 stub — full tool catalog lands in Phase 6. The server already validates that `go test ./...` + `curl /healthz` plumbing works before we bind tools.
