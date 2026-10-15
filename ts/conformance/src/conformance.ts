// The conformance API: the contract (src/contract.ts) implemented with oRPC and served under /api/mock.
// The notes are a log (src/notes.ts: D1 and the hub in the Worker), which starts with five seeded
// notes; every call is allowed by any of the four ways in (charter-lib/auth); it records what the
// SDK sent (`seen`) so that the test in src/index.ts can check it.
import { OpenAPIHandler } from "@orpc/openapi/fetch";
import { call, implement, ORPCError, withEventMeta } from "@orpc/server";
import { ResponseHeadersPlugin } from "@orpc/server/plugins";
import { authorize, discoveryURL, securityOf, type Trusted } from "charter-lib/auth";
import { follow } from "charter-lib/follow";
import { mcpHandler } from "charter-lib/mcp";
import { keyOf, limitOf, rateLimit } from "charter-lib/ratelimit";
import { contract, document, END, info } from "./contract.ts";
import type { Notes } from "./notes.ts";

/** What a deployment gives the API: its notes' log, whom it trusts, the Rate Limiting bindings that
 * count its callers (the Worker's env; without one, calls are not limited), and where the noteCreated
 * webhook goes (none: nowhere). */
export interface Deployment {
	notes: Notes;
	trusted: Trusted;
	limits: object;
	webhook?: Webhook;
}

/** The settings (a deployment's secrets and variables): whom it trusts, and where its webhook goes. */
export type Settings = Partial<Record<"READ_TOKEN" | "WRITE_TOKEN" | "ACCESS_TEAM_DOMAIN" | "ACCESS_AUD" | "OIDC_ISSUER" | "OIDC_AUDIENCE" | "WEBHOOK_URL" | "WEBHOOK_SECRET", string>>;

/** Where the noteCreated webhook goes, and the secret it is signed with. */
export interface Webhook {
	url: string;
	secret: string;
}

/** The webhook a deployment's settings name: WEBHOOK_URL (unset: none), signed with WEBHOOK_SECRET
 * ("whsec" by default, as go/conformance has it, so the same test runs against either). */
export const webhookOf = (settings: Settings): Webhook | undefined =>
	settings.WEBHOOK_URL ? { url: settings.WEBHOOK_URL, secret: settings.WEBHOOK_SECRET || "whsec" } : undefined;

/** The header the signature goes in (the document's x-fern-webhook-signature). */
const signatureHeader = "x-webhook-signature";

/** notify sends the noteCreated webhook, signed: the header carries the HMAC-SHA256 of the exact body,
 * in hex, under the secret. The receiver checks it with the SDK's WebhooksHelper.verifySignature. */
async function notify(webhook: Webhook, payload: unknown): Promise<void> {
	const body = JSON.stringify(payload);
	const key = await crypto.subtle.importKey("raw", new TextEncoder().encode(webhook.secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
	const mac = new Uint8Array(await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(body)));
	const signature = [...mac].map(b => b.toString(16).padStart(2, "0")).join("");
	const res = await fetch(webhook.url, { method: "POST", headers: { "content-type": "application/json", [signatureHeader]: signature }, body, signal: AbortSignal.timeout(5000) });
	await res.body?.cancel();
	if (!res.ok) throw new Error(`${webhook.url} answered HTTP ${res.status}`);
}

/** The OAuth token /oauth/token issues: the one there is, as there is one client. */
const issuedToken = "tok-1";

/**
 * Who may do what, by every way in: the API's tokens (READ_TOKEN may only read), Cloudflare Access (a
 * person writes, a machine's service token reads), an OpenID Connect issuer (its token's scope claim),
 * and the OAuth tokens /oauth/token issues, which read and write. An unset setting trusts no one that way.
 */
export const trusted = (settings: Settings): Trusted => ({
	tokens: [
		{ secret: "WRITE_TOKEN", value: settings.WRITE_TOKEN, scopes: ["read", "write"] },
		{ secret: "READ_TOKEN", value: settings.READ_TOKEN, scopes: ["read"] },
	],
	access: { team: settings.ACCESS_TEAM_DOMAIN, aud: settings.ACCESS_AUD, people: ["read", "write"], machines: ["read"] },
	oidc: { issuer: settings.OIDC_ISSUER, audience: settings.OIDC_AUDIENCE },
	issued: { scheme: "OAuth", check: token => (token === issuedToken ? ["read", "write"] : undefined) },
});

/** A request's credentials: its headers, where on the channel a token sent as ?access_token= (Workers
 * and browsers can't set WebSocket headers) is the Authorization header. */
function credentials(request: Request): Headers {
	const url = new URL(request.url), token = url.searchParams.get("access_token");
	if (!token || request.headers.has("authorization") || !url.pathname.endsWith("/notes/live")) return request.headers;
	const headers = new Headers(request.headers);
	headers.set("authorization", `Bearer ${token}`);
	return headers;
}

export const seen = { tokenCalls: 0, tokenContentType: "", authOnCreate: "", idempotencyKey: "" };

/** The one topic the WebSocket channel has. */
const topic = "notes";

// What the contract doesn't describe travels in the request: the credentials and the idempotency key
// are headers the SDK adds by itself (the document's security schemes and x-fern-idempotency-headers).
// Every call is allowed as its contract's security says (the document's, read, when it says none):
// 401 without credentials it knows, 403 without the scope; then limited as its contract says, per
// caller (429 with Retry-After: writeLimit). The router is built from `os`, without the checks: oRPC
// would otherwise run them twice per call.
const os = implement(contract).$context<{ request: Request; resHeaders?: Headers } & Deployment>();
const api = os.use(async ({ context, procedure, next }) => {
	const headers = credentials(context.request);
	const decision = await authorize(headers, securityOf(procedure, document.security), context.trusted);
	if (decision.status !== 200) throw new ORPCError(decision.status === 401 ? "UNAUTHORIZED" : "FORBIDDEN", { message: decision.message });
	const verdict = await rateLimit(limitOf(procedure), keyOf(decision.caller, headers), context.limits);
	if (verdict.status === 429) {
		context.resHeaders?.set("Retry-After", String(verdict.retryAfter));
		throw new ORPCError("TOO_MANY_REQUESTS", { message: verdict.message });
	}
	return next();
});

/** The token as the client sent it. The SDK sends a header (Node); Workers and browsers can't set WebSocket headers, so it may come as ?access_token= instead. */
function authorization(request: Request) {
	const token = new URL(request.url).searchParams.get("access_token");
	return request.headers.get("authorization") ?? (token ? `Bearer ${token}` : "");
}

export const router = os.router({
	auth: {
		getToken: api.auth.getToken.handler(({ input, context }) => {
			seen.tokenCalls++;
			seen.tokenContentType = context.request.headers.get("content-type") ?? "";
			if (input.client_id !== "id-1" || input.client_secret !== "secret-1") throw new ORPCError("UNAUTHORIZED", { message: "bad client" });
			return { access_token: "tok-1", expires_in: 3600 };
		}),
	},
	notes: {
		// Cursors are opaque strings to callers (here, how many notes came before).
		list: api.notes.list.handler(async ({ input, context }) => {
			const start = Number(input.cursor ?? 0), limit = input.limit ?? 2;
			if (!Number.isInteger(start) || start < 0) throw new ORPCError("BAD_REQUEST", { message: "not a cursor from next_cursor" });
			// One more than the page: whether there is a next one.
			const page = await context.notes.page(start, limit + 1);
			return { data: page.slice(0, limit), next_cursor: page.length > limit ? String(start + limit) : undefined };
		}),
		// Idempotent by its key: the note's id is the Idempotency-Key, and the log keeps the first note
		// under it, so a retry gets the first answer and announces nothing. A new note goes to the hub.
		create: api.notes.create.handler(async ({ input, context }) => {
			seen.authOnCreate = authorization(context.request);
			seen.idempotencyKey = context.request.headers.get("idempotency-key") ?? "";
			const { entry, created } = await context.notes.put({ id: seen.idempotencyKey || `note-${crypto.randomUUID()}`, body: input.body });
			if (created) {
				// The hub is only a wake-up: if it misses this note, followers find it in the log.
				await context.notes.publish(entry).catch(error => console.warn(`create: note ${entry.note.id} is not announced`, String(error)));
				if (context.webhook) await notify(context.webhook, { event: "note.created", note: entry.note }).catch(error => console.warn(`create: note ${entry.note.id}: the webhook was not delivered`, String(error)));
			}
			return entry.note;
		}),
		// SSE over follow(): every event's id is its place in the log, so `after` and Last-Event-ID are
		// the same position (a reconnecting SDK resends its original `after` plus a newer Last-Event-ID:
		// the newer wins). A planned end (`seconds`) returns END, the terminator; if follow() gives up
		// (hub down) the stream just ends without it, so the SDKs reconnect by themselves.
		watch: api.notes.watch.handler(async function* ({ input, context, signal, lastEventId }) {
			const until = AbortSignal.any([AbortSignal.timeout(input.seconds * 1000), ...(signal ? [signal] : [])]);
			const positions = [input.after, lastEventId].filter((v): v is string => !!v && /^\d+$/.test(v)).map(Number);
			try {
				const after = positions.length ? Math.max(...positions) : undefined;
				for await (const { id, note } of follow(context.notes, { after, signal: until, onBroken: error => console.warn("watch: hub subscription broken, resubscribing", String(error)) })) {
					yield withEventMeta(note, { id: String(id), retry: 1000 });
				}
			} catch (error) {
				console.error("watch: follow gave up, ending without terminator", String(error));
				return undefined;
			}
			return END;
		}),
		// The WebSocket channel's two streams; live() below carries them over the socket. The feed is
		// the log from its start, then each new note as the hub announces it (follow(), as in the notes
		// example); what the client sends is checked as it comes: the one topic is taken, any other
		// ends the stream with an error, which closes the socket with 1008.
		live: api.notes.live.handler(async function* ({ input, context, signal }) {
			const auth = authorization(context.request);
			const stop = new AbortController();
			signal?.addEventListener("abort", () => stop.abort());
			let refused: unknown;
			void (async () => {
				try {
					for await (const message of input) {
						if (message.topic !== topic) throw new ORPCError("BAD_REQUEST", { message: `no topic ${message.topic}: the one topic is ${topic}` });
					}
				} catch (error) {
					refused = error;
					stop.abort();
				}
			})();
			for await (const { note } of follow(context.notes, { after: 0, signal: stop.signal, onBroken: error => console.warn("live: hub subscription broken, resubscribing", String(error)) })) {
				yield { event: `${topic}.created`, id: note.id, body: note.body, auth };
			}
			if (refused) throw refused;
		}),
	},
	files: {
		// Zod types a file by its `type` and `size` only; under Workers' types it is the global File.
		upload: api.files.upload.handler(({ input }) => ({ id: `${(input.file as File).name}:${input.note ?? ""}`, size: input.file.size })),
	},
	chat: api.chat.handler(async function* ({ input }) {
		const words = `echo ${input.prompt}`.split(" ");
		for (const [i, text] of words.entries()) {
			yield { text, done: i === words.length - 1 };
			await new Promise(r => setTimeout(r, 10));
		}
	}),
});

const handler = new OpenAPIHandler(router, { plugins: [new ResponseHeadersPlugin()] });
// The contract's procedures that answer once, as MCP tools: a call runs in the REST handler's context.
const mcp = mcpHandler(router, { info });

// WebSocket transport for the contract's notes.live (AsyncAPI channel liveNotes): the socket's
// messages are the procedure's input stream, each validated by the contract, and what it yields is
// sent as plain JSON. A message the contract or the channel refuses closes the socket with 1008.
async function live(request: Request, deployment: Deployment): Promise<Response> {
	if (request.headers.get("upgrade") !== "websocket") return new Response("expected a WebSocket upgrade", { status: 426 });
	// Refused before the upgrade: no socket opens.
	const decision = await authorize(credentials(request), securityOf(router.notes.live, document.security), deployment.trusted);
	if (decision.status !== 200) return new Response(decision.message, { status: decision.status, headers: decision.status === 401 ? { "www-authenticate": "Bearer" } : {} });
	const [client, server] = Object.values(new WebSocketPair());
	server.accept();
	const closed = new AbortController();
	const messages = new ReadableStream<{ topic: string }>({
		start(controller) {
			server.addEventListener("message", event => {
				try { controller.enqueue(JSON.parse(String(event.data))); } catch (error) { controller.error(error); }
			});
			server.addEventListener("close", () => { closed.abort(); try { controller.close(); } catch {} });
		},
	});
	const events = await call(router.notes.live, messages.values(), { context: { request, ...deployment }, signal: closed.signal });
	void (async () => {
		try {
			for await (const event of events) server.send(JSON.stringify(event));
		} catch (error) {
			if (!closed.signal.aborted) server.close(1008, String((error as Error).message).slice(0, 100));
		}
	})();
	return new Response(null, { status: 101, webSocket: client });
}

/** The conformance API on a deployment, as a fetch function, so the SDK test can also call it without
 * the network. */
export async function mockApi(request: Request, deployment: Deployment): Promise<Response> {
	const path = new URL(request.url).pathname;
	if (path === "/api/mock/notes/live") return live(request, deployment);
	if (path === "/api/mock/mcp") return mcp(request, { request, ...deployment });
	// The specs' openIdConnectUrl: on to the issuer's discovery document.
	if (path === "/api/mock/.well-known/openid-configuration") {
		const to = discoveryURL(deployment.trusted.oidc?.issuer);
		return to ? Response.redirect(to, 302) : new Response("this API trusts no OpenID Connect issuer", { status: 404 });
	}
	const { matched, response } = await handler.handle(request, { prefix: "/api/mock", context: { request, ...deployment } });
	return matched ? response : new Response("not found", { status: 404 });
}
