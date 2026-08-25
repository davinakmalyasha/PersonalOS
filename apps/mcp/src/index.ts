#!/usr/bin/env node
// personal-os MCP server — stdio transport.
// Env: PERSONAL_OS_URL (default http://localhost:8080), PERSONAL_OS_TOKEN (matches API API_TOKEN).

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { PersonalOSClient } from "./api.js";
import { registerTools } from "./tools.js";

const baseUrl = process.env.PERSONAL_OS_URL ?? "http://localhost:8080";
const token = process.env.PERSONAL_OS_TOKEN;

async function main(): Promise<void> {
  const api = new PersonalOSClient(baseUrl, token);

  // Startup diagnostics to stderr (never stdout — stdio transport owns it).
  const healthy = await api.health();
  if (!healthy) {
    console.error(`[personal-os-mcp] WARNING: API not reachable at ${baseUrl}`);
    console.error("[personal-os-mcp] Start it with: go run ./services/api/cmd/api");
  } else if (!token) {
    console.error("[personal-os-mcp] WARNING: PERSONAL_OS_TOKEN not set; requests will 401 when the API requires a token.");
  } else {
    console.error(`[personal-os-mcp] API reachable at ${baseUrl}; token configured.`);
  }

  const server = new McpServer({
    name: "personal-os",
    version: "0.1.0",
  });

  registerTools(server, api);

  await server.connect(new StdioServerTransport());
  console.error("[personal-os-mcp] connected over stdio");
}

main().catch((err: unknown) => {
  console.error("[personal-os-mcp] fatal:", err instanceof Error ? err.message : err);
  process.exit(1);
});
