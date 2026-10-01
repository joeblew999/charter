// The showcase server (src/showcase.ts) without a Worker: the same fetch function the harness serves,
// called from Node. What the SDK does with it inside workerd is `mise run sdk:harness:test`.
//   mise run showcase:test
import assert from "node:assert/strict";
import { test } from "node:test";
import { call } from "@orpc/server";
import { mockApi, router, seen } from "../src/showcase.ts";

const api = (path: string, init?: RequestInit) => mockApi(new Request(`https://mock.invalid/api/mock${path}`, init));
const json = (body: unknown): RequestInit => ({ method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });

test("the token endpoint takes a form, and refuses a wrong client with 401", async () => {
	const token = await api("/oauth/token", { method: "POST", body: new URLSearchParams({ client_id: "id-1", client_secret: "secret-1" }) });
	assert.deepEqual(await token.json(), { access_token: "tok-1", expires_in: 3600 });
	assert.match(seen.tokenContentType, /application\/x-www-form-urlencoded/);
	const refused = await api("/oauth/token", { method: "POST", body: new URLSearchParams({ client_id: "id-1", client_secret: "wrong" }) });
	assert.equal(refused.status, 401);
});

test("notes come in pages of two, and limit changes that", async () => {
	assert.deepEqual(await (await api("/notes")).json(), { data: [{ id: "n1", body: "note n1" }, { id: "n2", body: "note n2" }], next_cursor: "2" });
	assert.deepEqual(await (await api("/notes?cursor=4")).json(), { data: [{ id: "n5", body: "note n5" }] });
	assert.equal((await (await api("/notes?limit=5")).json() as { data: unknown[] }).data.length, 5);
});

test("create takes its id from the Idempotency-Key and records the token", async () => {
	const created = await api("/notes", { ...json({ body: "hello" }), headers: { "content-type": "application/json", "idempotency-key": "key-1", authorization: "Bearer tok-1" } });
	assert.deepEqual(await created.json(), { id: "key-1", body: "hello" });
	assert.equal(seen.authOnCreate, "Bearer tok-1");
	assert.equal((await api("/notes", json({}))).status, 400);
});

test("an upload is multipart: the file and a field", async () => {
	const form = new FormData();
	form.set("file", new File(["hello"], "hello.txt"));
	form.set("note", "greeting");
	assert.deepEqual(await (await api("/files", { method: "POST", body: form })).json(), { id: "hello.txt:greeting", size: 5 });
	const none = new FormData();
	none.set("note", "greeting");
	assert.equal((await api("/files", { method: "POST", body: none })).status, 400);
});

test("chat streams one SSE event per word", async () => {
	const stream = await api("/chat", json({ prompt: "from node" }));
	assert.match(stream.headers.get("content-type") ?? "", /text\/event-stream/);
	const chunks = [...(await stream.text()).matchAll(/^data: (.+)$/gm)].map(match => JSON.parse(match[1]!));
	assert.deepEqual(chunks, [{ text: "echo", done: false }, { text: "from", done: false }, { text: "node", done: true }]);
});

test("the channel answers each Subscribe with three events, and rejects what the contract doesn't allow", async () => {
	const request = new Request("https://mock.invalid/api/mock/notes/live?access_token=tok-1");
	async function* messages(...sent: unknown[]) { yield* sent as { topic: string }[]; }
	const events = [];
	for await (const event of await call(router.notes.live, messages({ topic: "notes" }), { context: { request } })) events.push(event);
	assert.deepEqual(events.map(event => [event.event, event.id, event.auth]), [1, 2, 3].map(n => ["notes.created", `n${n}`, "Bearer tok-1"]));
	await assert.rejects(async () => { for await (const _ of await call(router.notes.live, messages({ subject: "notes" }), { context: { request } })); });
});

test("anything else is 404, and the channel without an upgrade is 426", async () => {
	assert.equal((await api("/nope")).status, 404);
	assert.equal((await api("/notes/live")).status, 426);
});
