# SDK — generated TS client

Generated from the Go API's canonical OpenAPI spec at `GET /openapi.json` (source file: `services/api/internal/server/openapi.json`).

## Generate (once OpenAPI stabilizes)

```bash
# example with openapi-typescript
npx openapi-typescript http://localhost:8080/openapi.json -o packages/sdk/src/client.ts
```

`apps/web` and `apps/mcp` consume this client instead of hand-rolled `fetch` shapes.

Phase 1: placeholder — the spec is served but generation lands after Finance shapes are stable.
