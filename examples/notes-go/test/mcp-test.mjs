// Test of the API's MCP endpoint (/api/mcp) with a real MCP client, the official TypeScript one, over
// Streamable HTTP. Everything runs twice, once per protocol era the server speaks: the stateless
// 2026-07-28 (every request names its version; server/discover) and the handshake one (initialize,
// 2025-11-25). The tools must be the contract's operations, and a tool call must do what the REST
// route does. Usage, from the project's folder (@modelcontextprotocol/client comes from its
// node_modules): node test/mcp-test.mjs <origin>
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
  check(label("connects"), client.getNegotiatedProtocolVersion() === version && client.getServerVersion()?.name === "charter-notes-go", [client.getNegotiatedProtocolVersion(), client.getServerVersion()]);

  // The tools are the contract's operations that answer once; their schemas are the contract's.
  const { tools } = await client.listTools();
  const byName = Object.fromEntries(tools.map(tool => [tool.name, tool]));
  // The notes tools are among them, each once; a stream and a channel never are. (Not "is exactly": a project adds its own.)
  const names = tools.map(tool => tool.name);
  check(label("tools/list has hello, listNotes, createNote, and no stream or channel"), ["hello", "listNotes", "createNote"].every(name => names.filter(n => n === name).length === 1) && !names.includes("watchNotes") && !names.includes("liveNotes"), names);
  const limit = byName.listNotes?.inputSchema.properties?.limit;
  check(label("listNotes takes the query parameters"), limit?.minimum === 1 && limit?.maximum === 100 && limit?.default === 20 && "cursor" in byName.listNotes.inputSchema.properties, byName.listNotes?.inputSchema);
  check(label("createNote takes the JSON body's properties"), byName.createNote?.inputSchema.required?.join() === "body" && byName.createNote.inputSchema.properties.body.minLength === 1, byName.createNote?.inputSchema);
  check(label("listNotes says what it returns"), byName.listNotes?.outputSchema?.properties?.data?.items?.$ref === "#/$defs/Note" && !!byName.listNotes.outputSchema.$defs?.Note, byName.listNotes?.outputSchema);

  // Calls run the operations: the client checks structuredContent against outputSchema itself.
  const hello = await client.callTool({ name: "hello", arguments: {} });
  check(label("hello"), !hello.isError && hello.structuredContent?.message?.startsWith("Hello from"), hello);

  const body = `mcp ${era} ${Date.now()}`;
  const created = await client.callTool({ name: "createNote", arguments: { body } });
  const note = created.structuredContent;
  check(label("createNote"), !created.isError && note?.body === body && Number.isInteger(note?.id) && text(created) === JSON.stringify(note), created);

  // The note is the REST API's note, and the tool's answer is the REST route's answer.
  const rest = await (await fetch(`${origin}/api/notes?limit=3`)).json();
  const listed = await client.callTool({ name: "listNotes", arguments: { limit: 3 } });
  check(label("listNotes answers what GET /api/notes answers"), !listed.isError && rest.data[0]?.id === note?.id && JSON.stringify(listed.structuredContent) === JSON.stringify(rest), { listed, rest });

  // What the contract refuses is a tool error with Huma's problem (so a model can correct itself).
  const refused = await client.callTool({ name: "listNotes", arguments: { limit: 1000 } });
  const problem = refused.isError ? JSON.parse(text(refused)) : null;
  check(label("validation failure is isError with the location"), problem?.status === 422 && problem.errors?.[0]?.location === "query.limit", refused);
  const empty = await client.callTool({ name: "createNote", arguments: { body: "" } });
  check(label("a refused body is isError with the location"), empty.isError && text(empty).includes('"location":"body.body"'), empty);

  // What is not a tool is a protocol error. A stream (watchNotes) is not a tool.
  for (const name of ["nope", "watchNotes"]) {
    const error = await client.callTool({ name, arguments: {} }).then(result => result, error => error);
    check(label(`unknown tool ${name} is error -32602`), error?.code === -32602, { code: error?.code, message: error?.message ?? error });
  }
  await client.close();
}

// Left to itself, the client finds the stateless era (it probes with server/discover).
const auto = new Client({ name: "mcp-test", version: "1.0.0" }, { versionNegotiation: { mode: "auto" } });
await auto.connect(new StreamableHTTPClientTransport(new URL(endpoint)));
check("auto negotiation picks 2026-07-28", auto.getNegotiatedProtocolVersion() === "2026-07-28" && auto.getProtocolEra() === "modern", [auto.getNegotiatedProtocolVersion(), auto.getProtocolEra()]);
await auto.close();

// The transport's edges, raw: no stream to GET, and JSON-RPC's own errors.
const post = body => fetch(endpoint, { method: "POST", headers: { "content-type": "application/json", accept: "application/json, text/event-stream" }, body });
const get = await fetch(endpoint, { headers: { accept: "text/event-stream" } });
check("GET is 405 (no stream)", get.status === 405 && get.headers.get("allow") === "POST", get.status);
const malformed = await post('{"jsonrpc":"2.0","id":1,"method":');
check("malformed JSON is error -32700", malformed.status === 400 && (await malformed.json()).error?.code === -32700, malformed.status);
const unknown = await (await post('{"jsonrpc":"2.0","id":1,"method":"resources/list"}')).json();
check("unknown method is error -32601", unknown.error?.code === -32601 && unknown.id === 1, unknown);
const notified = await post('{"jsonrpc":"2.0","method":"notifications/initialized"}');
check("a notification is 202 with no body", notified.status === 202 && (await notified.text()) === "", notified.status);

process.exit(failed ? 1 : 0);
