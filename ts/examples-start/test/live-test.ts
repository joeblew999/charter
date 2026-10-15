// Live test of the API: what a deploy must pass, run against a local server or the deployed Worker.
// What it gets back is checked against the contract's schemas. Add a check for each route you add.
// Usage, from the project's folder: node test/live-test.ts <origin>
import { hello } from "../src/contract.ts";

const origin = process.argv[2];
if (!origin) {
	console.log("FAIL  usage: node test/live-test.ts <origin>");
	process.exit(1);
}
let failed = 0;
function check(name: string, ok: boolean, detail: unknown): void {
	console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok ? "" : `: ${JSON.stringify(detail)}`}`);
	if (!ok) failed++;
}

const res = await fetch(`${origin}/api/hello`);
const body = hello.safeParse(await res.json().catch(() => null));
check("GET /api/hello", res.status === 200 && body.success && body.data.message.startsWith("Hello from "), { status: res.status, body });

const spec = await fetch(`${origin}/api/openapi.json`);
const openapi = (await spec.json().catch(() => null)) as { servers?: { url: string }[] } | null;
check("GET /api/openapi.json names this origin", spec.status === 200 && openapi?.servers?.[0]?.url === origin, { status: spec.status, servers: openapi?.servers });

// The routes as MCP tools (charter-lib/serve): an agent finds hello at /api/mcp.
const meta = { "io.modelcontextprotocol/protocolVersion": "2026-07-28", "io.modelcontextprotocol/clientCapabilities": {}, "io.modelcontextprotocol/clientInfo": { name: "live-test", version: "1" } };
const listed = await fetch(`${origin}/api/mcp`, {
	method: "POST",
	headers: { "content-type": "application/json", accept: "application/json, text/event-stream", "mcp-protocol-version": "2026-07-28", "mcp-method": "tools/list" },
	body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "tools/list", params: { _meta: meta } }),
});
const tools = ((await listed.json().catch(() => null)) as { result?: { tools?: { name: string }[] } } | null)?.result?.tools?.map(tool => tool.name);
check("POST /api/mcp lists hello as a tool", listed.status === 200 && !!tools?.includes("hello"), { status: listed.status, tools });

process.exit(failed ? 1 : 0);
