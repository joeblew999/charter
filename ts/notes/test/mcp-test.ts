// The notes API's MCP endpoint (/api/mcp) with a real MCP client, the official TypeScript one, over
// Streamable HTTP, in both protocol eras: the stateless 2026-07-28 and the 2025-11-25 handshake. The
// tools are the contract's operations that answer once, and a tool call does what the REST route
// does, with its auth: the same test runs against go/notes (go/lib/humamcp) and ts/notes
// (charter-lib/mcp), so it checks what both must do, not either one's wording. The server's name is
// the API's own title, from its spec. Usage: node test/mcp-test.ts <origin> (WRITE_TOKEN set)
import { Client, StreamableHTTPClientTransport } from "@modelcontextprotocol/client";
import { accessHeaders } from "./access.ts";

const origin = process.argv[2];
if (!origin || !process.env.WRITE_TOKEN) {
	console.log("FAIL  usage: node test/mcp-test.ts <origin>, with WRITE_TOKEN set (fnox exec --)");
	process.exit(1);
}
const endpoint = new URL(`${origin}/api/mcp`);
const edge = accessHeaders(origin);
const title = ((await (await fetch(`${origin}/api/openapi.json`, { headers: edge })).json()) as { info: { title: string } }).info.title;
let failed = 0;
function check(name: string, ok: boolean, detail: unknown): void {
	console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok ? "" : `: ${typeof detail === "string" ? detail : JSON.stringify(detail)}`}`);
	if (!ok) failed++;
}
type Schema = { type?: string; properties?: Record<string, Schema>; items?: Schema; required?: string[]; minimum?: number; maximum?: number; default?: unknown; minLength?: number; $ref?: string; $defs?: Record<string, Schema> };
type Tool = { name: string; inputSchema: Schema; outputSchema?: Schema };
type Result = { content: { type: string; text?: string }[]; structuredContent?: Record<string, unknown>; isError?: boolean };
const text = (result: Result) => result.content.map(block => block.text ?? "").join("");
/** A schema, or what its $ref names in the tool's $defs. */
const resolved = (schema: Schema | undefined, root: Schema | undefined): Schema | undefined =>
	schema?.$ref ? root?.$defs?.[schema.$ref.replace("#/$defs/", "")] : schema;

for (const [era, mode, version] of [["stateless", { pin: "2026-07-28" }, "2026-07-28"], ["handshake", "legacy", "2025-11-25"]] as const) {
	const client = new Client({ name: "mcp-test", version: "1.0.0" }, { versionNegotiation: { mode } });
	// A tool call runs the REST operation with this Authorization: createNote needs the write token.
	await client.connect(new StreamableHTTPClientTransport(endpoint, { requestInit: { headers: { authorization: `Bearer ${process.env.WRITE_TOKEN}`, ...edge } } }));
	const label = (name: string) => `${era} (${client.getNegotiatedProtocolVersion()}): ${name}`;
	check(label("connects"), client.getNegotiatedProtocolVersion() === version && client.getServerVersion()?.name === title, [client.getNegotiatedProtocolVersion(), client.getServerVersion()]);

	// The tools are the contract's operations that answer once: a stream or a channel never is one.
	const tools = (await client.listTools()).tools as Tool[];
	const byName = Object.fromEntries(tools.map(tool => [tool.name, tool]));
	const names = tools.map(tool => tool.name);
	check(label("tools/list has hello, listNotes, createNote, and no stream or channel"), ["hello", "listNotes", "createNote"].every(name => names.filter(n => n === name).length === 1) && !names.includes("watchNotes") && !names.includes("liveNotes") && !names.includes("receiveNote"), names);
	const limit = byName.listNotes?.inputSchema.properties?.limit;
	check(label("listNotes takes the query parameters"), limit?.minimum === 1 && limit?.maximum === 100 && limit?.default === 20 && "cursor" in (byName.listNotes?.inputSchema.properties ?? {}), byName.listNotes?.inputSchema);
	check(label("createNote takes the JSON body's properties"), byName.createNote?.inputSchema.required?.join() === "body" && byName.createNote.inputSchema.properties?.body?.minLength === 1, byName.createNote?.inputSchema);
	const item = resolved(byName.listNotes?.outputSchema?.properties?.data?.items, byName.listNotes?.outputSchema);
	check(label("listNotes says what it returns"), ["id", "body", "created_at"].every(field => field in (item?.properties ?? {})), byName.listNotes?.outputSchema);

	// Calls run the operations: the client checks structuredContent against outputSchema itself.
	const hello = (await client.callTool({ name: "hello", arguments: {} })) as Result;
	check(label("hello"), !hello.isError && String(hello.structuredContent?.message).startsWith("Hello from"), hello);

	const body = `mcp ${era} ${Date.now()}`;
	const created = (await client.callTool({ name: "createNote", arguments: { body } })) as Result;
	const note = created.structuredContent;
	check(label("createNote"), !created.isError && note?.body === body && Number.isInteger(note?.id), created);

	// The note is the REST API's note, and the tool's answer is the REST route's answer.
	const rest: unknown = await (await fetch(`${origin}/api/notes?limit=3`, { headers: edge })).json();
	const listed = (await client.callTool({ name: "listNotes", arguments: { limit: 3 } })) as Result;
	check(label("listNotes answers what GET /api/notes answers"), !listed.isError && (rest as { data: { id: number }[] }).data[0]?.id === note?.id && JSON.stringify(listed.structuredContent) === JSON.stringify(rest), { listed, rest });

	// What the contract refuses is a tool error that names what was wrong, so a model can correct itself.
	const refused = (await client.callTool({ name: "listNotes", arguments: { limit: 1000 } })) as Result;
	check(label("a refused argument is isError, naming it"), refused.isError === true && text(refused).includes("limit"), refused);
	const empty = (await client.callTool({ name: "createNote", arguments: { body: "" } })) as Result;
	check(label("a refused body is isError, naming it"), empty.isError === true && text(empty).includes("body"), empty);

	// What is not a tool is a protocol error. A stream (watchNotes) is not a tool.
	for (const name of ["nope", "watchNotes"]) {
		const error = (await client.callTool({ name, arguments: {} }).then(result => result, (e: unknown) => e)) as { code?: number; message?: string };
		check(label(`unknown tool ${name} is error -32602`), error?.code === -32602, { code: error?.code, message: error?.message ?? error });
	}
	await client.close();
}

// Left to itself, the client finds the stateless era (it probes with server/discover).
const auto = new Client({ name: "mcp-test", version: "1.0.0" }, { versionNegotiation: { mode: "auto" } });
await auto.connect(new StreamableHTTPClientTransport(endpoint, { requestInit: { headers: edge } }));
check("auto negotiation picks 2026-07-28", auto.getNegotiatedProtocolVersion() === "2026-07-28", auto.getNegotiatedProtocolVersion());
await auto.close();

// The transport's edges, raw: JSON-RPC's own errors, and a notification has no answer. Streamable
// HTTP lets a server answer a request as JSON or as one Server-Sent Event (the client takes both).
const post = (body: string) => fetch(endpoint, { method: "POST", headers: { "content-type": "application/json", accept: "application/json, text/event-stream", ...edge }, body });
const answer = async (response: Response): Promise<{ id?: number; error?: { code: number } }> => {
	const body = await response.text();
	const json = response.headers.get("content-type")?.startsWith("text/event-stream") ? (/^data: ?(.*)$/m.exec(body)?.[1] ?? "") : body;
	return JSON.parse(json) as { id?: number; error?: { code: number } };
};
const malformed = await post('{"jsonrpc":"2.0","id":1,"method":');
check("malformed JSON is error -32700", (await answer(malformed)).error?.code === -32700, malformed.status);
const unknown = await answer(await post('{"jsonrpc":"2.0","id":1,"method":"nope/nope"}'));
check("an unknown method is error -32601", unknown.error?.code === -32601 && unknown.id === 1, unknown);
const notified = await post('{"jsonrpc":"2.0","method":"notifications/initialized"}');
check("a notification is 202 with no body", notified.status === 202 && (await notified.text()) === "", notified.status);

process.exit(failed ? 1 : 0);
