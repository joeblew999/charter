import { env } from "cloudflare:workers";
import { OpenAPIHandler } from "@orpc/openapi/fetch";
import { DurablePublisher } from "@orpc/cloudflare";
import { call, implement, ORPCError, withEventMeta } from "@orpc/server";
import { ResponseHeadersPlugin } from "@orpc/server/plugins";
import type { z } from "zod";
import { asyncInfo, contract, END, info, type note } from "./contract.ts";
import { asyncapiSpec, openapiSpec } from "@charter/ts/specs";
import { follow, type FollowSource } from "@charter/ts/follow";
import { accessScheme, authorize, discoveryURL, oidcScheme, scheme, securityOf, type Caller } from "@charter/ts/auth";
import { keyOf, limitOf, rateLimit } from "@charter/ts/ratelimit";
export { NotesHub } from "./hub.ts";

type Note = z.infer<typeof note>;

// New notes go through oRPC's publisher on the NotesHub Durable Object (live fan-out only).
const publisher = () => new DurablePublisher<{ note: Note }>(env.HUB);

// The feed both transports serve (docs/guides/streaming.md): D1 is the log, the hub only wakes followers.
const notes: FollowSource<Note> = {
	subscribe: (listener, onError) => publisher().subscribe("note", listener, { onError }),
	since: async (after, limit) =>
		(await env.DB.prepare("SELECT id, body, created_at FROM notes WHERE id > ? ORDER BY id LIMIT ?").bind(after, limit).all<Note>()).results,
	latest: async () => (await env.DB.prepare("SELECT COALESCE(MAX(id), 0) AS id FROM notes").first<{ id: number }>())!.id,
};

/** The one feed: notes after `after` (a note id string), or from now; hub drops are logged, not seen. */
const followNotes = (after: string | undefined, signal: AbortSignal | undefined) =>
	follow(notes, { after: after === undefined ? undefined : Number(after), signal, onBroken: error => console.warn("follow: hub subscription broken, resubscribing", String(error)) });

// Who may do what (the contract says what each operation needs; reads are public, writing needs write):
//   - the Worker's secrets, each with its scopes: READ_TOKEN may not write;
//   - Cloudflare Access, once mise run access:setup has put it in front of the Worker: a person who
//     logged in writes, a machine's service token reads (and writes with the write token beside it);
//   - an OpenID Connect issuer (OIDC_ISSUER, OIDC_AUDIENCE): what each token's scope claim says.
// An unset secret or setting trusts no one. The four settings are optional secrets
// (cloudflare.config.ts), so the generated Env may not name them.
const settings = env as typeof env & Partial<Record<"ACCESS_TEAM_DOMAIN" | "ACCESS_AUD" | "OIDC_ISSUER" | "OIDC_AUDIENCE", string>>;
const trusted = () => ({
	tokens: [
		{ secret: "WRITE_TOKEN", value: env.WRITE_TOKEN, scopes: ["read", "write"] },
		{ secret: "READ_TOKEN", value: env.READ_TOKEN, scopes: ["read"] },
	],
	access: { team: settings.ACCESS_TEAM_DOMAIN, aud: settings.ACCESS_AUD, people: ["read", "write"], machines: ["read"] },
	oidc: { issuer: settings.OIDC_ISSUER, audience: settings.OIDC_AUDIENCE },
});

// The specs' security schemes (spec.ts writes the same).
const base = oidcScheme(accessScheme(info.title, scheme()));

// The contract (src/contract.ts) implemented on D1, served by oRPC's OpenAPIHandler as plain REST,
// each call allowed by the credentials it carries (401 without any it knows, 403 without the scope),
// then limited as its contract says, per caller (429 with Retry-After: writeLimit); the handler finds
// its caller in the context. The router is built from `os`, without the middleware: oRPC would
// otherwise run it twice per call, once for the router and once for the procedure.
const os = implement(contract).$context<{ headers?: Headers; resHeaders?: Headers; caller?: Caller }>();
const api = os
	.use(async ({ context, procedure, next }) => {
		const headers = context.headers ?? new Headers();
		const decision = await authorize(headers, securityOf(procedure), trusted());
		if (decision.status !== 200) throw new ORPCError(decision.status === 401 ? "UNAUTHORIZED" : "FORBIDDEN", { message: decision.message });
		const verdict = await rateLimit(limitOf(procedure), keyOf(decision.caller, headers), env);
		if (verdict.status === 429) {
			context.resHeaders?.set("Retry-After", String(verdict.retryAfter));
			throw new ORPCError("TOO_MANY_REQUESTS", { message: verdict.message });
		}
		return next({ context: { caller: decision.caller } });
	});

export const router = os.router({
	hello: api.hello.handler(() => ({ message: `Hello from ${env.APP_NAME}` })),
	notes: {
		list: api.notes.list.handler(async ({ input }) => {
			// Cursors are opaque strings to callers (here, the last id seen).
			const cursor = input.cursor ? Number(input.cursor) : Number.MAX_SAFE_INTEGER;
			const { results } = await env.DB.prepare("SELECT id, body, created_at FROM notes WHERE id < ? ORDER BY id DESC LIMIT ?")
				.bind(cursor, input.limit + 1)
				.all<Note>();
			const page = results.slice(0, input.limit);
			return { data: page, next_cursor: results.length > input.limit ? String(page.at(-1)!.id) : undefined };
		}),
		// SSE over follow(): every event's id is the note id, so `after` and Last-Event-ID are the same
		// position (a reconnecting SDK resends its original `after` plus a newer Last-Event-ID: the
		// newer wins). A planned end (`seconds`) returns END, the terminator; if follow() gives up (hub
		// down) the stream just ends without it, so the SDKs reconnect by themselves. No error events:
		// generated clients would read them as notes.
		// Upstream: fern-api/fern#17938 (when fixed: an error event could tell clients why the stream ended)
		watch: api.notes.watch.handler(async function* ({ input, signal, lastEventId }) {
			const until = AbortSignal.any([AbortSignal.timeout(input.seconds * 1000), ...(signal ? [signal] : [])]);
			const positions = [input.after, lastEventId].filter((v): v is string => !!v && /^\d+$/.test(v)).map(Number);
			const from = positions.length ? String(Math.max(...positions)) : undefined;
			try {
				for await (const note of followNotes(from, until)) yield withEventMeta(note, { id: String(note.id), retry: 1000 });
			} catch (error) {
				console.error("watch: follow gave up, ending without terminator", String(error));
				return undefined;
			}
			return END;
		}),
		// The WebSocket channel's feed; the Worker's live() below carries it over the socket.
		live: api.notes.live.handler(async function* ({ input, signal }) {
			yield* followNotes(input.after, signal);
		}),
		create: api.notes.create.handler(async ({ input }) => {
			const note = await env.DB.prepare("INSERT INTO notes (body) VALUES (?) RETURNING id, body, created_at").bind(input.body).first<Note>();
			await publisher().publish("note", note!);
			return note!;
		}),
	},
});

// ResponseHeadersPlugin: a 429's Retry-After.
const handler = new OpenAPIHandler(router, { plugins: [new ResponseHeadersPlugin()] });

// WebSocket transport for the contract's notes.live (AsyncAPI channel liveNotes): the procedure is
// called with the query as its input (validated by the contract, 400 before upgrading), and each
// note it yields is sent as plain JSON. The Worker holds the socket; the hub behind follow()
// hibernates. If follow() gives up (hub down), the socket is closed with 1011 so the client
// reconnects with `after`: it never stays open with nothing behind it.
async function live(request: Request): Promise<Response> {
	if (request.headers.get("upgrade") !== "websocket") return new Response("expected a WebSocket upgrade", { status: 426 });
	const closed = new AbortController();
	let feed: AsyncIterable<Note>;
	try {
		feed = await call(router.notes.live, Object.fromEntries(new URL(request.url).searchParams), { signal: closed.signal, context: {} });
	} catch (error) {
		return new Response(`bad request: ${(error as Error).message}`, { status: 400 });
	}
	const [client, server] = Object.values(new WebSocketPair());
	server.accept();
	server.addEventListener("close", () => closed.abort());
	server.addEventListener("error", () => closed.abort());
	void (async () => {
		try {
			for await (const note of feed) server.send(JSON.stringify(note));
		} catch (error) {
			if (closed.signal.aborted) return;
			console.error("live: follow gave up", String(error));
			try { server.close(1011, "hub unavailable; reconnect with after"); } catch {}
		}
	})();
	return new Response(null, { status: 101, webSocket: client });
}

export default {
	async fetch(request) {
		const url = new URL(request.url);
		// The specs, generated from the same router as fern/{openapi,asyncapi}.json.
		if (url.pathname === "/api/openapi.json") return Response.json(await openapiSpec(router, { info, server: url.origin, base }));
		if (url.pathname === "/api/asyncapi.json") return Response.json(await asyncapiSpec(router, { info: asyncInfo, server: url.origin }));
		// The specs' openIdConnectUrl: on to the issuer's discovery document.
		if (url.pathname === "/.well-known/openid-configuration") {
			const to = discoveryURL(settings.OIDC_ISSUER);
			return to ? Response.redirect(to, 302) : new Response("this API trusts no OpenID Connect issuer", { status: 404 });
		}
		if (url.pathname === "/api/notes/live") return live(request);
		const { matched, response } = await handler.handle(request, { context: { headers: request.headers } });
		return matched ? response : new Response("not found", { status: 404 });
	},
} satisfies ExportedHandler;
