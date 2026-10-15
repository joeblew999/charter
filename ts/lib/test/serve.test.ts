// charter-lib/serve: an API served the standard way. What it routes where, the guard's refusals and
// their challenge, and what an API that trusts no one serves. The WebSocket channel needs workerd's
// WebSocketPair: the conformance projects' tests carry one both ways.
import { asyncIteratorObject, oc } from "@orpc/contract";
import { openapi } from "@orpc/openapi";
import { implement } from "@orpc/server";
import { describe, expect, it } from "vitest";
import { z } from "zod";
import { asyncapi } from "../src/asyncapi.ts";
import { needs, scheme, type Trusted } from "../src/auth.ts";
import { guard, serve, type Served } from "../src/serve.ts";

const note = z.object({ id: z.number(), body: z.string() });
const contract = {
	list: oc.meta(openapi({ method: "GET", path: "/api/notes", operationId: "listNotes" })).output(z.object({ data: z.array(note) })),
	create: oc.meta(openapi({ method: "POST", path: "/api/notes", operationId: "createNote", spec: needs(["write"]) })).input(z.object({ body: z.string().min(1) })).output(z.object({ note, by: z.string() })),
	live: oc.meta(asyncapi({ channel: "liveNotes", address: "/api/notes/live" })).input(z.object({ after: z.string().optional() })).output(asyncIteratorObject(note)),
};
const os = implement(contract).$context<Served>();
const api = os.use(guard);
const router = os.router({
	list: api.list.handler(() => ({ data: [{ id: 1, body: "one" }] })),
	create: api.create.handler(({ input, context }) => ({ note: { id: 2, body: input.body }, by: context.caller?.token ?? "" })),
	live: api.live.handler(async function* () {}),
});
const document = { info: { title: "Notes", version: "1.0.0" }, asyncInfo: { title: "Notes live", version: "1.0.0" }, base: scheme() };
const trusted: Trusted = { tokens: [{ secret: "WRITE_TOKEN", value: "w", scopes: ["read", "write"] }, { secret: "READ_TOKEN", value: "r", scopes: ["read"] }] };
const served = serve(router, { document, trusted: () => trusted });
const origin = "https://notes.example";
const get = (path: string, init?: RequestInit) => served(new Request(`${origin}${path}`, init), {});
const create = (authorization?: string) =>
	get("/api/notes", { method: "POST", headers: { "content-type": "application/json", ...(authorization && { authorization }) }, body: JSON.stringify({ body: "hi" }) });

describe("serve", () => {
	it("serves both specs, with the request's origin as their server", async () => {
		const openapi = (await (await get("/api/openapi.json")).json()) as { servers: { url: string }[]; paths: Record<string, unknown> };
		expect(openapi.servers[0]?.url).toBe(origin);
		expect(Object.keys(openapi.paths)).toEqual(["/api/notes"]);
		const asyncapi = (await (await get("/api/asyncapi.json")).json()) as { channels: Record<string, { address: string }> };
		expect(asyncapi.channels.liveNotes?.address).toBe("/api/notes/live");
	});
	it("answers what the contract says, public where it needs nothing", async () => {
		expect(await (await get("/api/notes")).json()).toEqual({ data: [{ id: 1, body: "one" }] });
		expect((await get("/api/nope")).status).toBe(404);
	});
	it("refuses a call without credentials with 401, whose challenge points to the API's metadata", async () => {
		const refused = await create();
		expect(refused.status).toBe(401);
		expect(refused.headers.get("www-authenticate")).toBe(`Bearer resource_metadata="${origin}/.well-known/oauth-protected-resource"`);
	});
	it("refuses a caller without the scope with 403, naming it", async () => {
		const refused = await create("Bearer r");
		expect(refused.status).toBe(403);
		expect(refused.headers.get("www-authenticate")).toContain('error="insufficient_scope", scope="write"');
	});
	it("lets a caller with the scope in, and the handler knows who", async () => {
		const created = await create("Bearer w");
		expect(created.status).toBe(200);
		expect(await created.json()).toEqual({ note: { id: 2, body: "hi" }, by: "WRITE_TOKEN" });
	});
	it("says how to get in at /.well-known/", async () => {
		const metadata = (await (await get("/.well-known/oauth-protected-resource")).json()) as { resource: string; scopes_supported: string[] };
		expect(metadata).toMatchObject({ resource: origin, scopes_supported: ["write"] });
	});
	it("serves the tools at /api/mcp, with the REST route's guard", async () => {
		const call = await get("/api/mcp", {
			method: "POST",
			headers: { "content-type": "application/json", accept: "application/json, text/event-stream", "mcp-protocol-version": "2026-07-28", "mcp-method": "tools/call", "mcp-name": "createNote" },
			body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "tools/call", params: { name: "createNote", arguments: { body: "hi" }, _meta: { "io.modelcontextprotocol/protocolVersion": "2026-07-28", "io.modelcontextprotocol/clientCapabilities": {}, "io.modelcontextprotocol/clientInfo": { name: "test", version: "1" } } } }),
		});
		expect(call.status).toBe(401);
		expect(call.headers.get("www-authenticate")).toContain("resource_metadata=");
	});
	it("serves a channel at its address, and only as a WebSocket", async () => {
		expect((await get("/api/notes/live")).status).toBe(426);
	});
	it("checks no credentials and says nothing at /.well-known/ when it trusts no one", async () => {
		const open = serve(router, { document });
		expect((await open(new Request(`${origin}/.well-known/oauth-protected-resource`), {})).status).toBe(404);
		const created = await open(new Request(`${origin}/api/notes`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ body: "hi" }) }), {});
		expect(created.status).toBe(200);
	});
});
