// The conformance API (handlers.ts) without a Worker: the same fetch function the Worker serves,
// called from Node, on a log in memory (store.ts). What the SDK does with it inside workerd, on D1
// and the hub, is `npm run test:workerd`.
//   node --test api/handlers.test.ts   (npm run check runs it)
import assert from "node:assert/strict";
import { test } from "node:test";
import { call } from "@orpc/server";
import { type Deployment, handler, newToken, oauthOf, router, seen, trusted } from "./handlers.ts";
import { memoryNotes } from "./store.ts";

const settings = { READ_TOKEN: "r", WRITE_TOKEN: "w" };
const deployment: Deployment = { notes: memoryNotes(), trusted: trusted(settings), oauth: oauthOf(settings), limits: {} };
/** A token the API issued, as the SDKs get one. */
const issued = await newToken(oauthOf(settings).tokenSecret);
/** A call, with the OAuth token /api/oauth/token issues unless it sends credentials of its own. */
const api = (path: string, init: RequestInit = {}) => {
	const headers = new Headers(init.headers);
	if (!headers.has("authorization")) headers.set("authorization", `Bearer ${issued}`);
	return handler(new Request(`https://conformance.invalid${path}`, { ...init, headers }), deployment);
};
const as = (token: string) => ({ authorization: `Bearer ${token}` });
const json = (body: unknown): RequestInit => ({ method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });

test("the token endpoint takes a form, and refuses a wrong client with 401", async () => {
	const token = await api("/api/oauth/token", { method: "POST", body: new URLSearchParams({ client_id: "id-1", client_secret: "secret-1" }) });
	const answer = (await token.json()) as { access_token: string };
	assert.match(answer.access_token, /^\d+\.[0-9a-f]{64}$/);
	assert.deepEqual(answer, { access_token: answer.access_token, token_type: "Bearer", expires_in: 3600 });
	assert.match(seen.tokenContentType, /application\/x-www-form-urlencoded/);
	const refused = await api("/api/oauth/token", { method: "POST", body: new URLSearchParams({ client_id: "id-1", client_secret: "wrong" }) });
	assert.equal(refused.status, 401);
	// client_secret_basic, as an OAuth client (an MCP client among them) sends it.
	const basic = await api("/api/oauth/token", { method: "POST", headers: { authorization: `Basic ${btoa("id-1:secret-1")}` }, body: new URLSearchParams({ grant_type: "client_credentials", scope: "read write" }) });
	assert.equal(((await basic.json()) as { token_type: string }).token_type, "Bearer");
});

test("notes come in pages of two, and limit changes that", async () => {
	assert.deepEqual(await (await api("/api/notes")).json(), { data: [{ id: "n1", body: "note n1" }, { id: "n2", body: "note n2" }], next_cursor: "2" });
	assert.deepEqual(await (await api("/api/notes?cursor=4")).json(), { data: [{ id: "n5", body: "note n5" }] });
	assert.equal((await (await api("/api/notes?limit=5")).json() as { data: unknown[] }).data.length, 5);
});

test("create takes its id from the Idempotency-Key, keeps the first answer under it, and records the token", async () => {
	const create = (body: string) => api("/api/notes", { ...json({ body }), headers: { "content-type": "application/json", "idempotency-key": "key-1", authorization: `Bearer ${issued}` } });
	assert.deepEqual(await (await create("hello")).json(), { id: "key-1", body: "hello" });
	assert.deepEqual(await (await create("something else")).json(), { id: "key-1", body: "hello" });
	assert.equal(seen.authOnCreate, `Bearer ${issued}`);
	assert.deepEqual(await (await api("/api/notes?cursor=5")).json(), { data: [{ id: "key-1", body: "hello" }] });
	assert.equal((await api("/api/notes", json({}))).status, 422);
});

test("an upload is multipart: the file and a field", async () => {
	const form = new FormData();
	form.set("file", new File(["hello"], "hello.txt"));
	form.set("note", "greeting");
	assert.deepEqual(await (await api("/api/files", { method: "POST", body: form })).json(), { id: "hello.txt:greeting", size: 5 });
	const none = new FormData();
	none.set("note", "greeting");
	assert.equal((await api("/api/files", { method: "POST", body: none })).status, 422);
});

test("chat streams one SSE event per word", async () => {
	const stream = await api("/api/chat", json({ prompt: "from node" }));
	assert.match(stream.headers.get("content-type") ?? "", /text\/event-stream/);
	const chunks = [...(await stream.text()).matchAll(/^data: (.+)$/gm)].map(match => JSON.parse(match[1]!));
	assert.deepEqual(chunks, [{ text: "echo", done: false }, { text: "from", done: false }, { text: "node", done: true }]);
});

test("watch streams the log after a position, then ends at `seconds` with the terminator; the newer of after and Last-Event-ID wins", async () => {
	const request = new Request("https://conformance.invalid/api/notes/watch", { headers: as(issued) });
	const read = async (input: { after?: string; seconds: number }, lastEventId?: string) => {
		const ids: string[] = [];
		const stream = await call(router.notes.watch, input, { context: { request, ...deployment }, lastEventId });
		for (;;) {
			const next = await stream.next();
			if (next.done) return { ids, end: next.value };
			ids.push(next.value.id);
		}
	};
	const resumed = await read({ after: "3", seconds: 1 });
	assert.deepEqual(resumed.ids.slice(0, 2), ["n4", "n5"]);
	assert.equal(resumed.end, "[end-of-stream]");
	assert.equal((await read({ after: "1", seconds: 1 }, "4")).ids[0], "n5");
	assert.equal((await api("/api/notes", json({ body: "a [end-of-stream] inside" }))).status, 422);
});

test("the channel's feed is the log from its start, then each new note through the hub; a topic it lacks ends it", async () => {
	const request = new Request("https://conformance.invalid/api/notes/live", { headers: as(issued) });
	const stop = new AbortController();
	// The client's messages: those given, then nothing until the test ends.
	async function* messages(...sent: unknown[]) {
		yield* sent as { topic: string }[];
		await new Promise(resolve => stop.signal.addEventListener("abort", resolve));
	}
	const events: { event: string; id: string; auth?: string }[] = [];
	let idle: ReturnType<typeof setTimeout> | undefined;
	for await (const event of await call(router.notes.live, messages({ topic: "notes" }), { context: { request, ...deployment }, signal: stop.signal })) {
		events.push(event);
		// Once the log is read, a new note reaches the feed through the hub.
		clearTimeout(idle);
		if (event.id === "live-1") break;
		idle = setTimeout(() => void api("/api/notes", { ...json({ body: "live" }), headers: { "content-type": "application/json", "idempotency-key": "live-1" } }), 100);
	}
	stop.abort();
	assert.deepEqual(events.slice(0, 3).map(event => [event.event, event.id, event.auth]), [1, 2, 3].map(n => ["notes.created", `n${n}`, `Bearer ${issued}`]));
	assert.equal(events.at(-1)?.id, "live-1");
	for (const refused of [{ subject: "notes" }, { topic: "other" }]) {
		await assert.rejects(async () => { for await (const _ of await call(router.notes.live, messages(refused), { context: { request, ...deployment } })); });
	}
});

test("anything else is 404, and the channel without an upgrade is 426", async () => {
	assert.equal((await api("/api/nope")).status, 404);
	assert.equal((await api("/api/notes/live")).status, 426);
});

test("every call needs credentials by one of the ways in: reading needs read, writing write (last: it adds a note)", async () => {
	const none = await handler(new Request("https://conformance.invalid/api/notes"), deployment);
	assert.equal(none.status, 401);
	// The challenge says where the API tells a client how to get in (RFC 9728), and a 403 what it lacks.
	assert.equal(none.headers.get("www-authenticate"), 'Bearer resource_metadata="https://conformance.invalid/.well-known/oauth-protected-resource"');
	const reader = await api("/api/notes", { ...json({ body: "x" }), headers: { "content-type": "application/json", ...as("r") } });
	assert.match(reader.headers.get("www-authenticate") ?? "", /^Bearer error="insufficient_scope", scope="write", resource_metadata=/);
	assert.equal((await api("/api/notes", { headers: as("never-issued") })).status, 401);
	assert.equal((await api("/api/notes", { headers: as("r") })).status, 200);
	assert.equal((await api("/api/notes", { ...json({ body: "x" }), headers: { "content-type": "application/json", ...as("r") } })).status, 403);
	assert.equal((await api("/api/notes", { ...json({ body: "x" }), headers: { "content-type": "application/json", ...as("w") } })).status, 200);
	// The channel lets in, or refuses, before it asks for the upgrade.
	assert.equal((await api("/api/notes/live", { headers: as("never-issued") })).status, 401);
});
