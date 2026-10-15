// The API's MCP endpoint (/api/mcp) with a real MCP client, the official TypeScript one, over
// Streamable HTTP, in both protocol eras: the stateless 2026-07-28 and the 2025-11-25 handshake. The
// tools are the contract's operations, and a tool call answers what the REST route answers. The same
// test runs against the Go start projects (go/examples-start, go/examples-start-htmx, go/examples-start-datastar) and ts/examples-start: the
// server's name is the API's own title, from its spec. Usage: node test/mcp-test.ts <origin>
import { Client, StreamableHTTPClientTransport } from "@modelcontextprotocol/client";

const origin = process.argv[2];
if (!origin) {
	console.log("FAIL  usage: node test/mcp-test.ts <origin>");
	process.exit(1);
}
const endpoint = new URL(`${origin}/api/mcp`);
const title = ((await (await fetch(`${origin}/api/openapi.json`)).json()) as { info: { title: string } }).info.title;
let failed = 0;
function check(name: string, ok: boolean, detail: unknown): void {
	console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok ? "" : `: ${typeof detail === "string" ? detail : JSON.stringify(detail)}`}`);
	if (!ok) failed++;
}
type Result = { content: { type: string; text?: string }[]; structuredContent?: unknown; isError?: boolean };
const text = (result: Result) => result.content.map(block => block.text ?? "").join("");

for (const [era, mode, version] of [["stateless", { pin: "2026-07-28" }, "2026-07-28"], ["handshake", "legacy", "2025-11-25"]] as const) {
	const client = new Client({ name: "mcp-test", version: "1.0.0" }, { versionNegotiation: { mode } });
	await client.connect(new StreamableHTTPClientTransport(endpoint));
	const label = (name: string) => `${era} (${client.getNegotiatedProtocolVersion()}): ${name}`;
	check(label("connects"), client.getNegotiatedProtocolVersion() === version && client.getServerVersion()?.name === title, [client.getNegotiatedProtocolVersion(), client.getServerVersion()]);

	const { tools } = await client.listTools();
	check(label("tools/list has hello"), tools.filter(tool => tool.name === "hello").length === 1, tools.map(tool => tool.name));

	const hello = (await client.callTool({ name: "hello", arguments: {} })) as Result;
	const rest: unknown = await (await fetch(`${origin}/api/hello`)).json();
	check(label("hello answers what GET /api/hello answers"), !hello.isError && JSON.stringify(hello.structuredContent) === JSON.stringify(rest), { hello, rest });

	const error = (await client.callTool({ name: "nope", arguments: {} }).then(result => result, (e: unknown) => e)) as { code?: number; message?: string };
	check(label("an unknown tool is error -32602"), error?.code === -32602, { code: error?.code, message: error?.message ?? error });
	await client.close();
}

process.exit(failed ? 1 : 0);
