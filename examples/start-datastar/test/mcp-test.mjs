// Test of the API's MCP endpoint (/api/mcp) with a real MCP client, the official TypeScript one, over
// Streamable HTTP, in both protocol eras the server speaks: the stateless 2026-07-28 and the
// handshake one (2025-11-25). The tools are the contract's operations, and a tool call answers what
// the REST route answers. Usage, from the project's folder (@modelcontextprotocol/client comes from
// its node_modules): node test/mcp-test.mjs <origin>
import { createRequire } from "node:module";
const { Client, StreamableHTTPClientTransport } = createRequire(`${process.cwd()}/`)("@modelcontextprotocol/client");

const origin = process.argv[2];
const endpoint = `${origin}/api/mcp`;
let failed = 0;
function check(name, ok, detail) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok ? "" : `: ${typeof detail === "string" ? detail : JSON.stringify(detail)}`}`);
  if (!ok) failed++;
}
const text = result => result.content.map(block => block.text).join("");

for (const [era, mode, version] of [["stateless", { pin: "2026-07-28" }, "2026-07-28"], ["handshake", "legacy", "2025-11-25"]]) {
  const client = new Client({ name: "mcp-test", version: "1.0.0" }, { versionNegotiation: { mode } });
  await client.connect(new StreamableHTTPClientTransport(new URL(endpoint)));
  const label = name => `${era} (${client.getNegotiatedProtocolVersion()}): ${name}`;
  check(label("connects"), client.getNegotiatedProtocolVersion() === version && client.getServerVersion()?.name === "charter-start-datastar", [client.getNegotiatedProtocolVersion(), client.getServerVersion()]);

  const { tools } = await client.listTools();
  check(label("tools/list has hello"), tools.filter(tool => tool.name === "hello").length === 1, tools.map(tool => tool.name));

  const hello = await client.callTool({ name: "hello", arguments: {} });
  const rest = await (await fetch(`${origin}/api/hello`)).json();
  check(label("hello answers what GET /api/hello answers"), !hello.isError && JSON.stringify(hello.structuredContent) === JSON.stringify(rest) && text(hello) === JSON.stringify(rest), { hello, rest });

  const error = await client.callTool({ name: "nope", arguments: {} }).then(result => result, error => error);
  check(label("an unknown tool is error -32602"), error?.code === -32602, { code: error?.code, message: error?.message ?? error });
  await client.close();
}

process.exit(failed ? 1 : 0);
