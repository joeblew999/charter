// charter-lib/mcp: which procedures are tools and what they say, and that a call runs the procedure
// with its middleware, through the official SDK's handler (the stateless 2026-07-28 exchange, and
// the 2025-11-25 one for what differs).
import { asyncIteratorObject, oc } from "@orpc/contract";
import { openapi } from "@orpc/openapi";
import { implement, ORPCError } from "@orpc/server";
import { describe, expect, it } from "vitest";
import { z } from "zod";
import { asyncapi } from "../src/asyncapi.ts";
import { mcpHandler, notATool, toolsOf } from "../src/mcp.ts";

// A named schema, as a contract's are: zod writes it as a $ref at the root.
const note = z.object({ id: z.number(), body: z.string() }).meta({ id: "Note" });
const contract = {
	list: oc.meta(openapi({ method: "GET", path: "/api/notes", summary: "List notes", operationId: "listNotes" })).input(z.object({ limit: z.number().int().min(1).max(100) })).output(z.object({ data: z.array(note) })),
	create: oc.meta(openapi({ method: "POST", path: "/api/notes", summary: "Create a note", operationId: "createNote" })).input(z.object({ body: z.string().min(1) })).output(note),
	remove: oc.meta(openapi({ method: "DELETE", path: "/api/notes/{id}", summary: "Delete a note", operationId: "deleteNote" })).input(z.object({ id: z.number() })).output(z.object({})),
	watch: oc.meta(openapi({ method: "GET", path: "/api/notes/watch", operationId: "watchNotes" })).output(asyncIteratorObject(note)),
	live: oc.meta(asyncapi({ channel: "liveNotes", address: "/api/notes/live" })).output(asyncIteratorObject(note)),
	hidden: oc.meta(openapi({ method: "GET", path: "/api/hidden", operationId: "hidden" })).meta(notATool()).output(z.object({})),
};
const os = implement(contract).$context<{ headers?: Headers }>();
// Writing needs the header "x-may-write": as a REST route's auth would refuse it.
const guarded = os.use(({ context, next }) => {
	if (!context.headers?.get("x-may-write")) throw new ORPCError("FORBIDDEN", { message: "needs x-may-write" });
	return next();
});
const router = os.router({
	list: os.list.handler(({ input }) => ({ data: [{ id: 1, body: "one" }].slice(0, input.limit) })),
	create: guarded.create.handler(({ input }) => ({ id: 2, body: input.body })),
	remove: os.remove.handler(() => ({})),
	watch: os.watch.handler(async function* () {}),
	live: os.live.handler(async function* () {}),
	hidden: os.hidden.handler(() => ({})),
});

describe("toolsOf", () => {
	const tools = toolsOf(router);
	it("is every procedure that answers once, by operationId: not a stream, a channel, or one left out", () => {
		expect(tools.map(tool => tool.name)).toEqual(["listNotes", "createNote", "deleteNote"]);
	});
	it("says what humamcp says: GET is read-only, every method but POST may overwrite or delete, PUT and DELETE are idempotent", () => {
		const byName = Object.fromEntries(tools.map(tool => [tool.name, tool.annotations]));
		expect(byName).toEqual({ listNotes: { readOnlyHint: true }, createNote: { readOnlyHint: false, destructiveHint: false, idempotentHint: false }, deleteNote: { readOnlyHint: false, destructiveHint: true, idempotentHint: true } });
	});
});

describe("mcpHandler", () => {
	const mcp = mcpHandler(router, { info: { title: "notes-test", version: "1.0.0" } });
	const meta = { "io.modelcontextprotocol/protocolVersion": "2026-07-28", "io.modelcontextprotocol/clientCapabilities": {}, "io.modelcontextprotocol/clientInfo": { name: "test", version: "1" } };
	const rpc = async (method: string, params: Record<string, unknown>, headers: Record<string, string> = {}) => {
		const request = new Request("http://localhost/api/mcp", {
			method: "POST",
			headers: { "content-type": "application/json", accept: "application/json, text/event-stream", "mcp-protocol-version": "2026-07-28", "mcp-method": method, ...(params.name ? { "mcp-name": String(params.name) } : {}), ...headers },
			body: JSON.stringify({ jsonrpc: "2.0", id: 1, method, params: { ...params, _meta: meta } }),
		});
		const response = await mcp(request, { headers: request.headers });
		return (await response.json()) as { result?: { tools?: { name: string }[]; isError?: boolean; structuredContent?: unknown; content?: { text: string }[] }; error?: { code: number } };
	};
	it("lists the tools", async () => {
		expect((await rpc("tools/list", {})).result?.tools?.map(tool => tool.name)).toEqual(["listNotes", "createNote", "deleteNote"]);
	});
	it("runs a call through the procedure: its answer is structuredContent", async () => {
		const { result } = await rpc("tools/call", { name: "listNotes", arguments: { limit: 1 } });
		expect(result?.isError).toBeFalsy();
		expect(result?.structuredContent).toEqual({ data: [{ id: 1, body: "one" }] });
	});
	it("runs the procedure's middleware: what it refuses is a tool error saying why", async () => {
		const refused = (await rpc("tools/call", { name: "createNote", arguments: { body: "x" } })).result;
		expect(refused?.isError).toBe(true);
		expect(refused?.content?.[0]?.text).toContain("needs x-may-write");
		const allowed = (await rpc("tools/call", { name: "createNote", arguments: { body: "x" } }, { "x-may-write": "yes" })).result;
		expect(allowed?.structuredContent).toEqual({ id: 2, body: "x" });
	});
	it("answers a call refused for its credentials with that 401 and its challenge, so an MCP client signs in", async () => {
		const challenge = 'Bearer resource_metadata="https://api.test/.well-known/oauth-protected-resource"';
		const locked = os.use(({ context, next }) => {
			if (!context.headers?.get("authorization")) throw new ORPCError("UNAUTHORIZED", { message: "credentials are required", data: { challenge } });
			return next();
		});
		const lockedRouter = os.router({ ...router, remove: locked.remove.handler(() => ({})) });
		const handler = mcpHandler(lockedRouter, { info: { title: "notes-test", version: "1.0.0" } });
		const ask = async (headers: Record<string, string> = {}) => {
			const request = new Request("http://localhost/api/mcp", {
				method: "POST",
				headers: { "content-type": "application/json", accept: "application/json, text/event-stream", "mcp-protocol-version": "2026-07-28", "mcp-method": "tools/call", "mcp-name": "deleteNote", ...headers },
				body: JSON.stringify({ jsonrpc: "2.0", id: 5, method: "tools/call", params: { name: "deleteNote", arguments: { id: 1 }, _meta: meta } }),
			});
			return handler(request, { headers: request.headers });
		};
		const refused = await ask();
		expect(refused.status).toBe(401);
		expect(refused.headers.get("www-authenticate")).toBe(challenge);
		expect(await refused.json()).toMatchObject({ jsonrpc: "2.0", id: 5, error: { code: -32001 } });
		expect((await ask({ authorization: "Bearer t" })).status).toBe(200);
	});

	it("answers a named object as itself on a 2025 connection too, not wrapped as { result }", async () => {
		const legacy = async (id: number, method: string, params: Record<string, unknown>) => {
			const request = new Request("http://localhost/api/mcp", {
				method: "POST",
				headers: { "content-type": "application/json", accept: "application/json, text/event-stream", "mcp-protocol-version": "2025-11-25", "x-may-write": "yes" },
				body: JSON.stringify({ jsonrpc: "2.0", id, method, params }),
			});
			// Streamable HTTP answers as JSON or as one Server-Sent Event; this era, as an event.
			const response = await mcp(request, { headers: request.headers });
			const text = await response.text();
			const json = response.headers.get("content-type")?.startsWith("text/event-stream") ? (/^data: ?(.*)$/m.exec(text)?.[1] ?? "") : text;
			return JSON.parse(json) as { result?: { tools?: { name: string; outputSchema?: { type?: string } }[]; structuredContent?: unknown } };
		};
		await legacy(1, "initialize", { protocolVersion: "2025-11-25", capabilities: {}, clientInfo: { name: "test", version: "1" } });
		const created = await legacy(2, "tools/call", { name: "createNote", arguments: { body: "x" } });
		expect(created.result?.structuredContent).toEqual({ id: 2, body: "x" });
		const listed = await legacy(3, "tools/list", {});
		expect(listed.result?.tools?.find(tool => tool.name === "createNote")?.outputSchema?.type).toBe("object");
	});
});
