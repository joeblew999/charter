// The conformance API's MCP endpoint (<api>/mcp) with a real MCP client, the official TypeScript one,
// over Streamable HTTP, in both protocol eras: the stateless 2026-07-28 and the 2025-11-25 handshake.
// The tools are the contract's operations that answer once, less the token endpoint and the upload;
// a tool call does what the REST route does, with the token the MCP request carries. The same test
// runs against go/conformance (go/lib/humamcp) and ts/conformance (charter-lib/mcp), so it checks
// what both must do, not either one's wording.
// Usage: node test/mcp-test.ts <api> (the origin of the Go server, <origin>/api/mock of the oRPC one)
import { Client, StreamableHTTPClientTransport } from "@modelcontextprotocol/client";

const api = process.argv[2];
if (!api) {
	console.log("FAIL  usage: node test/mcp-test.ts <api>");
	process.exit(1);
}
const endpoint = new URL(`${api}/mcp`);
let failed = 0;
function check(name: string, ok: boolean, detail: unknown): void {
	console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok ? "" : `: ${typeof detail === "string" ? detail : JSON.stringify(detail)}`}`);
	if (!ok) failed++;
}
type Schema = { type?: string; properties?: Record<string, Schema>; items?: Schema; required?: string[]; $ref?: string; $defs?: Record<string, Schema> };
type Tool = { name: string; inputSchema: Schema; outputSchema?: Schema; annotations?: { readOnlyHint?: boolean } };
type Result = { content: { type: string; text?: string }[]; structuredContent?: Record<string, unknown>; isError?: boolean };
const text = (result: Result) => result.content.map(block => block.text ?? "").join("");
/** A schema, or what its $ref names in the tool's $defs. */
const resolved = (schema: Schema | undefined, root: Schema | undefined): Schema | undefined =>
	schema?.$ref ? root?.$defs?.[schema.$ref.replace("#/$defs/", "")] : schema;

// A token as the SDKs get one (OAuth client credentials): every operation but the token endpoint needs it.
const issued = await fetch(`${api}/oauth/token`, { method: "POST", body: new URLSearchParams({ client_id: "id-1", client_secret: "secret-1" }) });
const { access_token } = (await issued.json()) as { access_token: string };
const authorization = `Bearer ${access_token}`;

for (const [era, mode, version] of [["stateless", { pin: "2026-07-28" }, "2026-07-28"], ["handshake", "legacy", "2025-11-25"]] as const) {
	const client = new Client({ name: "mcp-test", version: "1.0.0" }, { versionNegotiation: { mode } });
	await client.connect(new StreamableHTTPClientTransport(endpoint, { requestInit: { headers: { authorization } } }));
	const label = (name: string) => `${era} (${client.getNegotiatedProtocolVersion()}): ${name}`;
	check(label("connects, as the API's title"), client.getNegotiatedProtocolVersion() === version && client.getServerVersion()?.name === "Conformance", [client.getNegotiatedProtocolVersion(), client.getServerVersion()]);

	// The tools: no token endpoint, no upload, no stream (chat), no channel (liveNotes, subscribe).
	const tools = (await client.listTools()).tools as Tool[];
	const byName = Object.fromEntries(tools.map(tool => [tool.name, tool]));
	const names = tools.map(tool => tool.name).sort();
	check(label("tools/list is listNotes and createNote"), names.join() === "createNote,listNotes", names);
	check(label("listNotes is read-only, and takes the query parameters"), byName.listNotes?.annotations?.readOnlyHint === true && ["cursor", "limit"].every(name => name in (byName.listNotes?.inputSchema.properties ?? {})), byName.listNotes);
	check(label("createNote takes the JSON body's properties"), byName.createNote?.inputSchema.required?.join() === "body", byName.createNote?.inputSchema);
	const item = resolved(byName.listNotes?.outputSchema?.properties?.data?.items, byName.listNotes?.outputSchema);
	check(label("listNotes says what it returns"), ["id", "body"].every(field => field in (item?.properties ?? {})), byName.listNotes?.outputSchema);

	// Calls run the operations: the client checks structuredContent against outputSchema itself.
	const body = `mcp ${era} ${Date.now()}`;
	const created = (await client.callTool({ name: "createNote", arguments: { body } })) as Result;
	check(label("createNote"), !created.isError && created.structuredContent?.body === body && typeof created.structuredContent?.id === "string", created);

	// The tool's answer is the REST route's answer.
	const rest: unknown = await (await fetch(`${api}/notes?limit=3`, { headers: { authorization } })).json();
	const listed = (await client.callTool({ name: "listNotes", arguments: { limit: 3 } })) as Result;
	check(label("listNotes answers what GET /notes answers"), !listed.isError && JSON.stringify(listed.structuredContent) === JSON.stringify(rest), { listed: listed.structuredContent, rest });

	// What the contract refuses is a tool error that names what was wrong, so a model can correct itself.
	const refused = (await client.callTool({ name: "listNotes", arguments: { limit: "many" } })) as Result;
	check(label("a refused argument is isError, naming it"), refused.isError === true && text(refused).includes("limit"), refused);
	const empty = (await client.callTool({ name: "createNote", arguments: {} })) as Result;
	check(label("a missing body is isError, naming it"), empty.isError === true && text(empty).includes("body"), empty);

	// What is not a tool is a protocol error.
	for (const name of ["nope", "getToken", "uploadFile", "chat", "liveNotes"]) {
		const error = (await client.callTool({ name, arguments: {} }).then(result => result, (e: unknown) => e)) as { code?: number; message?: string };
		check(label(`${name} is no tool: error -32602`), error?.code === -32602, { code: error?.code, message: error?.message ?? error });
	}
	await client.close();
}

// Signing in, as MCP's authorization has it (RFC 9728, RFC 8414): a tool call without credentials is a
// 401 whose challenge points to the API's metadata, which names an authorization server (the API's
// own), whose metadata names its token endpoint. The chain is followed here as an OAuth client does,
// by hand: MCP's own client (ClientCredentialsProvider) refuses an authorization server without an
// authorization endpoint, which RFC 8414 does not ask of one with only client credentials
// (docs/upstream.md).
const meta = { "io.modelcontextprotocol/protocolVersion": "2026-07-28", "io.modelcontextprotocol/clientCapabilities": {}, "io.modelcontextprotocol/clientInfo": { name: "mcp-test", version: "1" } };
const refused = await fetch(endpoint, {
	method: "POST",
	headers: { "content-type": "application/json", accept: "application/json, text/event-stream", "mcp-protocol-version": "2026-07-28", "mcp-method": "tools/call", "mcp-name": "createNote" },
	body: JSON.stringify({ jsonrpc: "2.0", id: 9, method: "tools/call", params: { name: "createNote", arguments: { body: "no credentials" }, _meta: meta } }),
});
const challenge = refused.headers.get("www-authenticate") ?? "";
const metadataURL = /resource_metadata="([^"]+)"/.exec(challenge)?.[1];
check("a tool call without credentials is a 401 whose challenge points to the API's metadata", refused.status === 401 && !!metadataURL, { status: refused.status, challenge });
type Metadata = { resource?: string; authorization_servers?: string[]; scopes_supported?: string[]; issuer?: string; token_endpoint?: string };
const metadata = metadataURL ? ((await (await fetch(metadataURL)).json()) as Metadata) : undefined;
check("the API's metadata names it, its own authorization server first, and its scopes", metadata?.resource === api && metadata.authorization_servers?.[0] === api && ["read", "write"].every(scope => metadata.scopes_supported?.includes(scope)), metadata);
const issuer = new URL(api);
const server = (await (await fetch(`${issuer.origin}/.well-known/oauth-authorization-server${issuer.pathname === "/" ? "" : issuer.pathname}`)).json()) as Metadata;
check("its authorization server's metadata names the token endpoint", server.issuer === api && server.token_endpoint === `${api}/oauth/token`, server);
// The token endpoint, with the client's credentials as an OAuth client sends them first (Basic), the
// grant, the scopes the metadata names and the resource (RFC 8707).
const granted = server.token_endpoint
	? await fetch(server.token_endpoint, { method: "POST", headers: { authorization: `Basic ${btoa("id-1:secret-1")}` }, body: new URLSearchParams({ grant_type: "client_credentials", scope: (metadata?.scopes_supported ?? []).join(" "), resource: metadata?.resource ?? "" }) })
	: undefined;
const token = granted?.ok ? ((await granted.json()) as { access_token?: string; token_type?: string }) : undefined;
check("its token endpoint gives a bearer token for the client's credentials", token?.token_type === "Bearer" && !!token.access_token, token ?? granted?.status);
const signedIn = new Client({ name: "mcp-test", version: "1.0.0" });
await signedIn.connect(new StreamableHTTPClientTransport(endpoint, { requestInit: { headers: { authorization: `Bearer ${token?.access_token ?? ""}` } } }));
const withToken = (await signedIn.callTool({ name: "createNote", arguments: { body: `signed in ${Date.now()}` } })) as Result;
check("with that token, the tool call goes through", !withToken.isError && typeof withToken.structuredContent?.id === "string", withToken);
await signedIn.close();
// The same refusal in the 2025 era, where the answer comes as a Server-Sent Event.
const legacy = new Client({ name: "mcp-test", version: "1.0.0" }, { versionNegotiation: { mode: "legacy" } });
await legacy.connect(new StreamableHTTPClientTransport(endpoint));
const legacyRefusal = await legacy.callTool({ name: "createNote", arguments: { body: "no credentials" } }).then(() => "answered", (error: unknown) => String(error));
check("handshake (2025-11-25): a tool call without credentials is refused with the 401, not answered", legacyRefusal !== "answered" && /401|Unauthorized|credentials/i.test(legacyRefusal), legacyRefusal);
await legacy.close().catch(() => {});

// Left to itself, the client finds the stateless era (it probes with server/discover).
const auto = new Client({ name: "mcp-test", version: "1.0.0" }, { versionNegotiation: { mode: "auto" } });
await auto.connect(new StreamableHTTPClientTransport(endpoint, { requestInit: { headers: { authorization } } }));
check("auto negotiation picks 2026-07-28", auto.getNegotiatedProtocolVersion() === "2026-07-28", auto.getNegotiatedProtocolVersion());
await auto.close();

// The transport's edges, raw: JSON-RPC's own errors, and a notification has no answer. Streamable
// HTTP lets a server answer a request as JSON or as one Server-Sent Event (the client takes both).
const post = (body: string) => fetch(endpoint, { method: "POST", headers: { "content-type": "application/json", accept: "application/json, text/event-stream", authorization }, body });
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
