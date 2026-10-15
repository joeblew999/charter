import { env } from "cloudflare:workers";
import { implement, withEventMeta } from "@orpc/server";
import type { Trusted } from "charter-lib/auth";
import { guard, serve, type Served } from "charter-lib/serve";
import { contract, END } from "./contract.ts";
import { document } from "./spec.ts";
import { before, followNotes, insert } from "./store.ts";

// Who may do what (the contract says what each operation needs; reads are public, writing needs write):
//   - the Worker's secrets, each with its scopes: READ_TOKEN may not write;
//   - Cloudflare Access, once it is in front of the Worker: a person who
//     logged in writes, a machine's service token reads (and writes with the write token beside it);
//   - an OpenID Connect issuer (OIDC_ISSUER, OIDC_AUDIENCE): what each token's scope claim says.
// An unset secret or setting trusts no one. The four settings are optional secrets
// (cloudflare.config.ts), so the generated Env may not name them.
const settings = env as typeof env & Partial<Record<"ACCESS_TEAM_DOMAIN" | "ACCESS_AUD" | "OIDC_ISSUER" | "OIDC_AUDIENCE", string>>;
const trusted = (): Trusted => ({
	tokens: [
		{ secret: "WRITE_TOKEN", value: env.WRITE_TOKEN, scopes: ["read", "write"] },
		{ secret: "READ_TOKEN", value: env.READ_TOKEN, scopes: ["read"] },
	],
	access: { team: settings.ACCESS_TEAM_DOMAIN, aud: settings.ACCESS_AUD, people: ["read", "write"], machines: ["read"] },
	oidc: { issuer: settings.OIDC_ISSUER, audience: settings.OIDC_AUDIENCE },
});

// The contract (api/contract.ts) implemented on D1 (api/store.ts), and served the standard way (charter-lib/serve):
// the routes under /api, both specs, the routes as MCP tools at /api/mcp, how to get in at
// /.well-known/, and the channel at its address. Each call goes through the guard: allowed by the
// credentials it carries (401 without any it knows, 403 without the scope, each with the challenge an
// MCP client reads to sign in), then limited as its contract says, per caller (429 with Retry-After:
// writeLimit); the handler finds its caller in the context. The router is built from `os`, without
// the guard: oRPC would otherwise run it twice per call.
const os = implement(contract).$context<Served>();
const api = os.use(guard);

export const router = os.router({
	hello: api.hello.handler(() => ({ message: `Hello from ${env.APP_NAME}` })),
	notes: {
		list: api.notes.list.handler(async ({ input }) => {
			// Cursors are opaque strings to callers (here, the last id seen).
			const cursor = input.cursor ? Number(input.cursor) : Number.MAX_SAFE_INTEGER;
			const results = await before(cursor, input.limit + 1);
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
		// The WebSocket channel's feed, which serve() carries over the socket: if follow() gives up (hub
		// down), the socket closes with 1011 so the client reconnects with `after`: it never stays open
		// with nothing behind it.
		live: api.notes.live.handler(async function* ({ input, signal }) {
			yield* followNotes(input.after, signal);
		}),
		create: api.notes.create.handler(({ input }) => insert(input.body)),
	},
});

/** The API, as a fetch function: what the Worker (worker.ts) answers with. The Rate Limiting bindings
 * are the Worker's own (cloudflare.config.ts). */
export const handler = serve(router, { document, trusted, limits: () => env });
