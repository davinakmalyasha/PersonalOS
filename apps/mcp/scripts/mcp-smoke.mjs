// Acceptance driver: speaks raw JSON-RPC over the MCP server's stdio.
// Usage: node scripts/mcp-smoke.mjs
import { spawn } from "node:child_process";

const server = spawn(process.execPath, ["dist/index.js"], {
  cwd: new URL("..", import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1"),
  env: {
    ...process.env,
    PERSONAL_OS_URL: process.env.PERSONAL_OS_URL ?? "http://localhost:8080",
    PERSONAL_OS_TOKEN: process.env.PERSONAL_OS_TOKEN ?? "test-token-123",
  },
});

let buf = "";
const pending = new Map();
server.stdout.on("data", (chunk) => {
  buf += chunk.toString();
  let idx;
  while ((idx = buf.indexOf("\n")) >= 0) {
    const line = buf.slice(0, idx).trim();
    buf = buf.slice(idx + 1);
    if (!line) continue;
    try {
      const msg = JSON.parse(line);
      if (msg.id !== undefined && pending.has(msg.id)) {
        pending.get(msg.id)(msg);
        pending.delete(msg.id);
      }
    } catch {
      console.error("unparseable line:", line.slice(0, 200));
    }
  }
});
server.stderr.on("data", (c) => process.stderr.write("[srv] " + c.toString()));

function send(msg) {
  server.stdin.write(JSON.stringify(msg) + "\n");
}
function request(id, method, params) {
  return new Promise((resolve) => {
    pending.set(id, resolve);
    send({ jsonrpc: "2.0", id, method, params });
  });
}
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function main() {
  await sleep(300);
  const init = await request(1, "initialize", {
    protocolVersion: "2025-03-26",
    capabilities: {},
    clientInfo: { name: "smoke", version: "0" },
  });
  console.log("initialized:", init.result?.serverInfo?.name ?? "?");
  send({ jsonrpc: "2.0", method: "notifications/initialized" });

  const list = await request(2, "tools/list", {});
  const names = list.result.tools.map((t) => t.name);
  console.log(`tools/list: ${names.length} tools`);
  console.log(names.join(", "));

  async function call(id, name, args) {
    const res = await request(id, "tools/call", { name, arguments: args });
    if (res.result?.isError) throw new Error(`${name}: ${res.result.content[0].text}`);
    return JSON.parse(res.result.content[0].text);
  }

  // Acceptance flow 1: planner today.
  const today = await call(10, "planner_today", {});
  console.log("\nplanner_today → date:", today.date, "| due_today:", today.due_today.length,
    "| habits:", today.habits.length);

  // Acceptance flow 2: universal capture + search round-trip.
  await call(11, "save_item", {
    type: "warranty", title: "Headphones warranty",
    data: { expires: "2027-03-01" }, tags: ["gear"],
  });
  const found = await call(12, "search_items", { q: "warranty headphones" });
  console.log("search_items → found:", found.items.length > 0, "| top:", found.items[0]?.title);

  // Acceptance flow 3: knowledge search sees the mirrored note.
  const know = await call(13, "search_knowledge", { q: "handbook" });
  console.log("search_knowledge → found:", know.items.length > 0, "| top:", know.items[0]?.title);

  // Acceptance flow 4: habit checkoff toggle returns streaks.
  const habit = await call(14, "manage_habits", { action: "create", name: "Read", cadence: "daily" });
  const toggled = await call(15, "check_habit", { habit_id: habit.id });
  console.log("check_habit → done:", toggled.done, "| streak:", toggled.streaks.current);

  console.log("\nALL MCP SMOKE CHECKS PASSED");
  server.kill();
  process.exit(0);
}

main().catch((err) => {
  console.error("SMOKE FAILED:", err.message ?? err);
  server.kill();
  process.exit(1);
});
