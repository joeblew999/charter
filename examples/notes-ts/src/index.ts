import { env } from "cloudflare:workers";
import { OpenAPIHandler } from "@orpc/openapi/fetch";
import { DurablePublisher } from "@orpc/cloudflare";
import { call, implement, withEventMeta } from "@orpc/server";
import type { z } from "zod";
import { asyncInfo, contract, END, info, type note } from "./contract.ts";
import { asyncapiSpec, openapiSpec } from "@charter/ts/specs";
import { follow, type FollowSource } from "@charter/ts/follow";
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

// The contract (src/contract.ts) implemented on D1, served by oRPC's OpenAPIHandler as plain REST.
const api = implement(contract);

export const router = api.router({
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

const handler = new OpenAPIHandler(router);

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
		feed = await call(router.notes.live, Object.fromEntries(new URL(request.url).searchParams), { signal: closed.signal });
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
		if (url.pathname === "/api/openapi.json") return Response.json(await openapiSpec(router, { info, server: url.origin }));
		if (url.pathname === "/api/asyncapi.json") return Response.json(await asyncapiSpec(router, { info: asyncInfo, server: url.origin }));
		if (url.pathname === "/api/notes/live") return live(request);
		const { matched, response } = await handler.handle(request, { context: {} });
		return matched ? response : new Response("not found", { status: 404 });
	},
} satisfies ExportedHandler;
