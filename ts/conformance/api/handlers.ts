// The conformance API: the contract (api/contract.ts) implemented with oRPC and served the standard way
// (charter-lib/serve): the routes under /api, both specs, the routes as MCP tools at /api/mcp, how to
// get in at /.well-known/, and the WebSocket channel at its address. The notes are a log (api/store.ts;
// on Cloudflare, D1 and the hub: api/store_d1.ts), which starts with five seeded notes; every call is
// allowed by any of the four ways in (charter-lib/auth); it records what the SDK sent (`seen`) so that
// the harness (harness.ts) can check it.
import { implement, ORPCError, withEventMeta } from "@orpc/server";
import type { Trusted } from "charter-lib/auth";
import { follow } from "charter-lib/follow";
import { guard, refused, serve, type Served } from "charter-lib/serve";
import { contract, END } from "./contract.ts";
import { document } from "./spec.ts";
import type { Notes } from "./store.ts";

/** What a deployment gives the API: its notes' log, whom it trusts, the Rate Limiting bindings that
 * count its callers (the Worker's env; without one, calls are not limited), and where the noteCreated
 * webhook goes (none: nowhere). */
export interface Deployment {
	notes: Notes;
	trusted: Trusted;
	oauth: OAuth;
	limits: object;
	webhook?: Webhook;
}

/** The settings (a deployment's secrets and variables): whom it trusts, and where its webhook goes. */
export type Settings = Partial<Record<"READ_TOKEN" | "WRITE_TOKEN" | "ACCESS_TEAM_DOMAIN" | "ACCESS_AUD" | "OIDC_ISSUER" | "OIDC_AUDIENCE" | "WEBHOOK_URL" | "WEBHOOK_SECRET" | "CLIENT_ID" | "CLIENT_SECRET" | "TOKEN_SECRET", string>>;

/** The OAuth client the API knows, and the secret its tokens are signed with: its settings, or the
 * defaults go/conformance has, so the same tests run against either. A real deployment sets them. */
export interface OAuth {
	clientId: string;
	clientSecret: string;
	tokenSecret: string;
}

export const oauthOf = (settings: Settings): OAuth => ({
	clientId: settings.CLIENT_ID || "id-1",
	clientSecret: settings.CLIENT_SECRET || "secret-1",
	tokenSecret: settings.TOKEN_SECRET || "conformance-token-secret",
});

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
	const signature = await sign(webhook.secret, body);
	const res = await fetch(webhook.url, { method: "POST", headers: { "content-type": "application/json", [signatureHeader]: signature }, body, signal: AbortSignal.timeout(5000) });
	await res.body?.cancel();
	if (!res.ok) throw new Error(`${webhook.url} answered HTTP ${res.status}`);
}

/** sign is HMAC-SHA256 in hex: what signs both the access tokens and the webhooks (as go/conformance). */
async function sign(secret: string, message: string): Promise<string> {
	const key = await crypto.subtle.importKey("raw", new TextEncoder().encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
	return [...new Uint8Array(await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(message)))].map(b => b.toString(16).padStart(2, "0")).join("");
}

/** How long an access token is good for, in seconds. */
const tokenLife = 3600;

/** An access token that needs no storage: its expiry, signed, so any request can check it (a Worker
 * keeps nothing between requests). The same as go/conformance's. */
export async function newToken(secret: string, now = Date.now()): Promise<string> {
	const expiry = String(Math.floor(now / 1000) + tokenLife);
	return `${expiry}.${await sign(secret, `access-token.${expiry}`)}`;
}

/** Whether a token is one newToken made with this secret, and still good. */
export async function validToken(secret: string, token: string, now = Date.now()): Promise<boolean> {
	const [expiry = "", signature = "", ...more] = token.split(".");
	const want = await sign(secret, `access-token.${expiry}`);
	let same = signature.length === want.length && more.length === 0;
	for (let i = 0; i < want.length; i++) same &&= signature.charCodeAt(i) === want.charCodeAt(i);
	return same && /^\d+$/.test(expiry) && Math.floor(now / 1000) < Number(expiry);
}

/**
 * Who may do what, by every way in: the API's tokens (READ_TOKEN may only read), Cloudflare Access (a
 * person writes, a machine's service token reads), an OpenID Connect issuer (its token's scope claim),
 * and the OAuth tokens /api/oauth/token issues, which read and write. An unset setting trusts no one that way.
 */
export const trusted = (settings: Settings): Trusted => ({
	tokens: [
		{ secret: "WRITE_TOKEN", value: settings.WRITE_TOKEN, scopes: ["read", "write"] },
		{ secret: "READ_TOKEN", value: settings.READ_TOKEN, scopes: ["read"] },
	],
	access: { team: settings.ACCESS_TEAM_DOMAIN, aud: settings.ACCESS_AUD, people: ["read", "write"], machines: ["read"] },
	oidc: { issuer: settings.OIDC_ISSUER, audience: settings.OIDC_AUDIENCE },
	issued: { scheme: "OAuth", tokenPath: "/api/oauth/token", check: async token => ((await validToken(oauthOf(settings).tokenSecret, token)) ? ["read", "write"] : undefined) },
});

/** A client's id and secret from an Authorization: Basic header, each form-encoded before the base64
 * (RFC 6749, 2.3.1). */
function basicCredentials(request: Request): { id: string; secret: string } | undefined {
	const [scheme, encoded = ""] = (request.headers.get("authorization") ?? "").split(" ", 2);
	if (scheme?.toLowerCase() !== "basic") return undefined;
	try {
		const raw = atob(encoded.trim()), colon = raw.indexOf(":");
		if (colon < 0) return undefined;
		const unescape = (value: string) => decodeURIComponent(value.replace(/\+/g, " "));
		return { id: unescape(raw.slice(0, colon)), secret: unescape(raw.slice(colon + 1)) };
	} catch {
		return undefined;
	}
}

export const seen = { tokenCalls: 0, tokenContentType: "", authOnCreate: "", idempotencyKey: "", issued: "" };

/** The one topic the WebSocket channel has. */
const topic = "notes";

// What the contract doesn't describe travels in the request: the credentials and the idempotency key
// are headers the SDK adds by itself (the document's security schemes and x-fern-idempotency-headers).
// Every call goes through the guard (charter-lib/serve): allowed as its contract's security says (the
// document's, read, when it says none), 401 without credentials it knows, 403 without the scope, each
// with the challenge an MCP client reads to sign in; then limited as its contract says, per caller (429
// with Retry-After: writeLimit). The router is built from `os`, without the guard: oRPC would otherwise
// run it twice per call.
const os = implement(contract).$context<Served & Deployment>();
const api = os.use(guard);

/** The token as the client sent it: on the channel, serve() takes one sent as ?access_token= (Workers
 * and browsers can't set WebSocket headers) for the Authorization header. */
const authorization = (request: Request) => request.headers.get("authorization") ?? "";

export const router = os.router({
	auth: {
		// The client-credentials grant: the client's credentials from the form or from a Basic header.
		getToken: api.auth.getToken.handler(async ({ input, context }) => {
			seen.tokenCalls++;
			seen.tokenContentType = context.request.headers.get("content-type") ?? "";
			if (input.grant_type !== undefined && input.grant_type !== "client_credentials") throw new ORPCError("BAD_REQUEST", { message: "unsupported_grant_type: the one grant is client_credentials" });
			const client = basicCredentials(context.request) ?? { id: input.client_id, secret: input.client_secret };
			if (client.id !== context.oauth.clientId || client.secret !== context.oauth.clientSecret) throw new ORPCError("UNAUTHORIZED", { message: "bad client" });
			seen.issued = await newToken(context.oauth.tokenSecret);
			return { access_token: seen.issued, token_type: "Bearer", expires_in: tokenLife };
		}),
	},
	notes: {
		// Cursors are opaque strings to callers (here, how many notes came before).
		list: api.notes.list.handler(async ({ input, context }) => {
			const start = Number(input.cursor ?? 0), limit = input.limit;
			if (!/^\d+$/.test(input.cursor ?? "0")) throw refused("query.cursor", "not a cursor from next_cursor", input.cursor);
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
		// The WebSocket channel's two streams, which serve() carries over the socket. The feed is
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

/** The conformance API on a deployment, as a fetch function: what the Worker (worker.ts) answers with,
 * and what the harness's SDK calls without the network. */
export const handler = serve<Deployment>(router, { document, trusted: deployment => deployment.trusted, limits: deployment => deployment.limits });
