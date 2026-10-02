// The showcase API: the contract (src/contract.ts) implemented with oRPC and served under /api/mock.
// It keeps nothing but five fixed notes, and records what the SDK sent (`seen`) so that the test in
// src/index.ts can check it.
import { OpenAPIHandler } from "@orpc/openapi/fetch";
import { call, implement, ORPCError } from "@orpc/server";
import { contract } from "./contract.ts";

export const notes = ["n1", "n2", "n3", "n4", "n5"].map(id => ({ id, body: `note ${id}` }));
export const seen = { tokenCalls: 0, tokenContentType: "", authOnCreate: "", idempotencyKey: "" };

// What the contract doesn't describe travels in the request: the bearer token and the idempotency key
// are headers the SDK adds by itself (the document's security scheme and x-fern-idempotency-headers).
const api = implement(contract).$context<{ request: Request }>();

/** The token as the client sent it. The SDK sends a header (Node); Workers and browsers can't set WebSocket headers, so it may come as ?access_token= instead. */
function authorization(request: Request) {
	const token = new URL(request.url).searchParams.get("access_token");
	return request.headers.get("authorization") ?? (token ? `Bearer ${token}` : "");
}

export const router = api.router({
	auth: {
		getToken: api.auth.getToken.handler(({ input, context }) => {
			seen.tokenCalls++;
			seen.tokenContentType = context.request.headers.get("content-type") ?? "";
			if (input.client_id !== "id-1" || input.client_secret !== "secret-1") throw new ORPCError("UNAUTHORIZED", { message: "bad client" });
			return { access_token: "tok-1", expires_in: 3600 };
		}),
	},
	notes: {
		list: api.notes.list.handler(({ input }) => {
			const start = Number(input.cursor ?? 0);
			const next = start + (input.limit ?? 2);
			return { data: notes.slice(start, next), next_cursor: next < notes.length ? String(next) : undefined };
		}),
		create: api.notes.create.handler(({ input, context }) => {
			seen.authOnCreate = authorization(context.request);
			seen.idempotencyKey = context.request.headers.get("idempotency-key") ?? "";
			return { id: seen.idempotencyKey || "new", body: input.body };
		}),
		// The WebSocket channel's two streams; live() below carries them over the socket.
		live: api.notes.live.handler(async function* ({ input, context }) {
			const auth = authorization(context.request);
			for await (const { topic } of input) {
				for (const note of notes.slice(0, 3)) yield { event: `${topic}.created`, id: note.id, body: note.body, auth };
			}
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

const handler = new OpenAPIHandler(router);

// WebSocket transport for the contract's notes.live (AsyncAPI channel liveNotes): the socket's
// messages are the procedure's input stream, each validated by the contract, and what it yields is
// sent as plain JSON. A message the contract rejects closes the socket with 1008.
async function live(request: Request): Promise<Response> {
	if (request.headers.get("upgrade") !== "websocket") return new Response("expected a WebSocket upgrade", { status: 426 });
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
	const events = await call(router.notes.live, messages.values(), { context: { request }, signal: closed.signal });
	void (async () => {
		try {
			for await (const event of events) server.send(JSON.stringify(event));
		} catch (error) {
			if (!closed.signal.aborted) server.close(1008, String((error as Error).message).slice(0, 100));
		}
	})();
	return new Response(null, { status: 101, webSocket: client });
}

/** The showcase API as a fetch function, so the SDK test can also call it without the network. */
export async function mockApi(request: Request): Promise<Response> {
	if (new URL(request.url).pathname === "/api/mock/notes/live") return live(request);
	const { matched, response } = await handler.handle(request, { prefix: "/api/mock", context: { request } });
	return matched ? response : new Response("not found", { status: 404 });
}
