// The generated TypeScript SDK against a running conformance server, over the real network: every Fern
// feature the conformance API has must work. The same test runs against examples/conformance-go and this
// project's, each by its origin, each with the SDK made from its own specs: the one in the folder
// it runs in (sdk/out/typescript-dist). It is typed by this project's copy (client/), which the
// twins check (charter-ts twins) holds to the same specs.
// Usage: node test/sdk.ts <origin> [--webhooks <receiver>]
//   --webhooks  the receiver the server sends the noteCreated webhook to (test/webhook-receiver.ts):
//               its deliveries are read at <receiver>/deliveries
import { existsSync } from "node:fs";
import { pathToFileURL } from "node:url";
import RawWebSocket from "ws";
import type * as Sdk from "../client/index.mjs";
import type * as Webhooks from "../client/webhooks/index.mjs";

const args = process.argv.slice(2);
const option = (name: string) => (args.includes(name) ? args.splice(args.indexOf(name), 2)[1] : undefined);
const receiver = option("--webhooks");
const [base] = args;
const { READ_TOKEN, WRITE_TOKEN } = process.env;
if (!base || !READ_TOKEN || !WRITE_TOKEN) {
	console.log("FAIL  usage: node test/sdk.ts <origin> [--webhooks <receiver>], with READ_TOKEN and WRITE_TOKEN set");
	process.exit(1);
}
const out = pathToFileURL(`${process.cwd()}/sdk/out/`);
const { ConformanceClient } = (await import(new URL("typescript-dist/esm/index.mjs", out).href)) as typeof Sdk;
const { WebhooksHelper } = (await import(new URL("typescript-dist/esm/webhooks/index.mjs", out).href)) as typeof Webhooks;
type Note = Sdk.Conformance.Note;
type NoteEvent = Sdk.Conformance.NoteEvent;

const TIMEOUT = 15000;
const credentials = { clientId: "id-1", clientSecret: "secret-1" };
const webhookSecret = "whsec";
const wsBase = base.replace(/^http/, "ws");
const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

let failed = 0;
async function check(name: string, run: () => Promise<[boolean, unknown]>): Promise<void> {
	let ok = false, detail: unknown;
	try {
		[ok, detail] = await Promise.race([run(), new Promise<never>((_, reject) => setTimeout(() => reject(new Error(`nothing within ${TIMEOUT / 1000} s`)), TIMEOUT))]);
	} catch (error) {
		detail = String((error as Error)?.message ?? error);
	}
	console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok ? "" : `: ${typeof detail === "string" ? detail : JSON.stringify(detail)}`}`);
	if (!ok) failed++;
}

// Everything the SDK sends goes through here, so the test sees it as the server does.
const sent: { method: string; path: string; headers: Headers }[] = [];
const recordingFetch: typeof fetch = (input, init = {}) => {
	const request = new Request(input, init);
	sent.push({ method: request.method, path: new URL(request.url).pathname, headers: request.headers });
	return fetch(input, { ...init, signal: init.signal ?? AbortSignal.timeout(TIMEOUT) });
};
const requests = (method: string, path: string) => sent.filter(request => request.method === method && request.path === path);
// One environment: `base` is the REST API's origin, `production` (the AsyncAPI server's name) the WebSocket's.
// The SDK takes any of the API's ways in, each under its own name: here OAuth client credentials.
const environment = { base, production: wsBase };
const client = new ConformanceClient({ environment, oauth: credentials, fetch: recordingFetch });
const bearer = () => requests("POST", "/api/notes").at(-1)?.headers.get("authorization") ?? "";

/** What the webhook's receiver (a customer's, test/webhook-receiver.ts) has been sent. */
const deliveries = async () => (await (await fetch(`${receiver}/deliveries`)).json()) as { body: string; signature: string }[];

// The log keeps what earlier runs created (D1 under workerd and on Cloudflare): a run's own notes
// have keys of their own, and the checks read the seeded five at the log's start.
const run = crypto.randomUUID().slice(0, 8);
const key = `key-${run}`;

await check("pagination (auto-paging: the seeded five over 3 pages)", async () => {
	const ids: string[] = [];
	for await (const note of await client.notes.list()) if (ids.push(note.id) === 5) break;
	const pages = requests("GET", "/api/notes").length;
	return [ids.join() === "n1,n2,n3,n4,n5" && pages === 3, { ids, pages }];
});

await check("idempotent create (Idempotency-Key + bearer token sent; a retry gets the first answer)", async () => {
	const note = await client.notes.create({ body: "hello" }, { idempotencyKey: key });
	const again = await client.notes.create({ body: "something else" }, { idempotencyKey: key });
	const request = requests("POST", "/api/notes").at(-1);
	const ok = note.id === key && again.id === key && again.body === "hello" && request?.headers.get("idempotency-key") === key && bearer().startsWith("Bearer ");
	return [ok, { note, again, authorization: bearer() }];
});

await check("oauth client credentials (token fetched, form-encoded, reused)", async () => {
	const tokens = requests("POST", "/api/oauth/token");
	const contentType = tokens[0]?.headers.get("content-type") ?? "";
	const authorized = sent.filter(request => request.path !== "/api/oauth/token").every(request => request.headers.get("authorization") === bearer());
	return [tokens.length === 1 && contentType.includes("application/x-www-form-urlencoded") && authorized, { tokenCalls: tokens.length, contentType, authorized }];
});

await check("the SDK with a bearer token (one of the API's): the write token writes, the read token is refused (403)", async () => {
	const as = (token: string) => new ConformanceClient({ environment, bearer: { token } });
	const note = await as(WRITE_TOKEN).notes.create({ body: "by the write token" });
	const refused = await as(READ_TOKEN).notes.create({ body: "by the read token" }).then(() => undefined, (error: unknown) => error as Sdk.ConformanceError);
	return [note.body === "by the write token" && refused?.statusCode === 403, { note, refused: refused?.statusCode ?? refused }];
});

await check("SSE stream (typed chunks)", async () => {
	const chunks: Sdk.Conformance.Chunk[] = [];
	for await (const chunk of await client.chat({ prompt: "from the sdk" })) chunks.push(chunk);
	const text = chunks.map(chunk => chunk.text).join(" ");
	return [text === "echo from the sdk" && chunks.at(-1)?.done === true, chunks];
});

await check("resumable SSE (typed notes after a position, ended by the terminator)", async () => {
	const ids: string[] = [];
	const started = Date.now();
	for await (const note of await client.notes.watch({ after: "3", seconds: 1 })) ids.push((note as Note).id);
	const ms = Date.now() - started;
	// The terminator ends it: a resumable stream without one would reconnect, and never end.
	return [ids.slice(0, 2).join() === "n4,n5" && ms < 5000, { ids: ids.slice(0, 5), ms }];
});

await check("SSE resumes from the newer of after and Last-Event-ID (what a reconnecting SDK sends)", async () => {
	const response = await fetch(`${base}/api/notes/watch?after=1&seconds=1`, { headers: { authorization: bearer(), "last-event-id": "4" } });
	const ids = [...(await response.text()).matchAll(/^id: (\d+)$/gm)].map(match => match[1]);
	return [ids[0] === "5", ids.slice(0, 5)];
});

await check("multipart file upload", async () => {
	const stored = await client.files.uploadFile({ file: new File(["x".repeat(20000)], "hello.txt", { type: "text/plain" }), note: "a note" });
	return [stored.id === "hello.txt:a note" && stored.size === 20000, stored];
});

await check("webhook HMAC signature: the SDK's helper (valid accepted, forged rejected)", async () => {
	const body = JSON.stringify({ event: "note.created", note: { id: "n1", body: "note n1" } });
	const hex = async (secret: string) => {
		const signing = await crypto.subtle.importKey("raw", new TextEncoder().encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
		return Buffer.from(await crypto.subtle.sign("HMAC", signing, new TextEncoder().encode(body))).toString("hex");
	};
	const good = await WebhooksHelper.verifySignature(body, await hex(webhookSecret), webhookSecret);
	const bad = await WebhooksHelper.verifySignature(body, await hex("wrong"), webhookSecret);
	return [good && !bad, { good, bad }];
});

if (receiver) {
	await check("webhook delivered and signed by the server (the SDK's helper verifies it)", async () => {
		// One delivery for the note, none for the retry, which announces nothing. Other checks create notes too.
		const mine = async () => (await deliveries()).filter(d => (JSON.parse(d.body) as { note?: Note }).note?.id === key);
		for (let waited = 0; (await mine()).length < 1 && waited < 5000; waited += 100) await sleep(100);
		await sleep(300);
		const delivered = await mine();
		if (delivered.length !== 1) return [false, `${delivered.length} deliveries for ${key}, want 1`];
		const delivery = delivered[0]!;
		const payload = JSON.parse(delivery.body) as { event: string; note?: Note };
		const good = await WebhooksHelper.verifySignature(delivery.body, delivery.signature, webhookSecret);
		const bad = await WebhooksHelper.verifySignature(delivery.body, delivery.signature, "wrong");
		const tampered = await WebhooksHelper.verifySignature(delivery.body.replace("hello", "hullo"), delivery.signature, webhookSecret);
		const ok = good && !bad && !tampered && payload.event === "note.created" && payload.note?.id === key && payload.note?.body === "hello";
		return [ok, { payload, signature: delivery.signature, good, bad, tampered }];
	});
}

// The socket both ways: the client sends `subscribe` (typed); the server's feed is the log from its
// start (the seeded notes first), then each new note as the hub announces it.
type Socket = ReturnType<typeof client.liveNotes.connect> extends Promise<infer S> ? S : never;
const firstThree = (socket: Socket) => new Promise<NoteEvent[]>((resolve, reject) => {
	const events: NoteEvent[] = [];
	socket.on("message", message => { events.push(message as NoteEvent); if (events.length === 3) resolve(events.slice()); });
	socket.on("error", error => reject(new Error(String((error as { message?: string })?.message ?? error))));
	socket.on("close", event => reject(new Error(`closed ${(event as { code?: number })?.code ?? ""}`)));
});
const expected = (events: NoteEvent[], authorization: string) =>
	events.map(event => `${event.event} ${event.id}`).join() === "notes.created n1,notes.created n2,notes.created n3" && events.every(event => event.auth === authorization);

await check("websocket client (typed sendSubscribe, typed events, bearer token sent)", async () => {
	const socket = await client.liveNotes.connect({ reconnectAttempts: 0, connectionTimeoutInSeconds: 10 });
	const done = firstThree(socket);
	await socket.waitForOpen();
	socket.sendSubscribe({ topic: "notes" });
	const events = await done;
	socket.close();
	return [expected(events, bearer()), events];
});

await check("websocket: a note created while connected arrives through the hub, not the log's 30 s recheck", async () => {
	const socket = await client.liveNotes.connect({ reconnectAttempts: 0, connectionTimeoutInSeconds: 10 });
	const ids: string[] = [];
	let last = Date.now();
	const live = `live-${run}`;
	const done = new Promise<NoteEvent>((resolve, reject) => {
		socket.on("message", message => { const event = message as NoteEvent; ids.push(event.id); last = Date.now(); if (event.id === live) resolve(event); });
		socket.on("error", error => reject(new Error(String((error as { message?: string })?.message ?? error))));
	});
	await socket.waitForOpen();
	socket.sendSubscribe({ topic: "notes" });
	// The log first: once nothing has come for half a second, the feed is waiting on the hub.
	while (ids.length < 5 || Date.now() - last < 500) await sleep(50);
	const started = Date.now();
	await client.notes.create({ body: "live" }, { idempotencyKey: live });
	const arrived = await Promise.race([done, new Promise<never>((_, reject) => setTimeout(() => reject(new Error(`${live} did not arrive within 5 s`)), 5000))]);
	socket.close();
	return [arrived.id === live && arrived.body === "live", { after: `${Date.now() - started} ms`, seen: ids.length }];
});

await check("websocket with the token as ?access_token= (what a browser or a Worker can send)", async () => {
	const { access_token } = await client.auth.getToken({ client_id: credentials.clientId, client_secret: credentials.clientSecret });
	// A browser's WebSocket: no headers.
	const socket = new WebSocket(`${wsBase}/api/notes/live?access_token=${encodeURIComponent(access_token)}`);
	const events = await new Promise<NoteEvent[]>((resolve, reject) => {
		const events: NoteEvent[] = [];
		socket.addEventListener("open", () => socket.send(JSON.stringify({ topic: "notes" })));
		socket.addEventListener("message", message => { events.push(JSON.parse(String(message.data)) as NoteEvent); if (events.length === 3) resolve(events.slice()); });
		socket.addEventListener("error", () => reject(new Error("the socket failed")));
		socket.addEventListener("close", event => reject(new Error(`closed ${event.code} ${event.reason}`)));
	});
	socket.close();
	return [expected(events, `Bearer ${access_token}`), events];
});

await check("audiences: the public SDK has notes and auth, not the internal upload", async () => {
	const resource = (name: string) => existsSync(new URL(`typescript-public/api/resources/${name}`, out));
	if (!existsSync(new URL("typescript-public", out))) return [false, "no sdk/out/typescript-public: npm run sdk -- typescript-public"];
	return [resource("notes") && resource("auth") && !resource("files") && existsSync(new URL("typescript-dist/esm/api/resources/files", out)), { notes: resource("notes"), files: resource("files") }];
});

// The contract's security is enforced, not only declared.
const raw = (path: string, init: RequestInit = {}) => fetch(`${base}${path}`, { ...init, signal: AbortSignal.timeout(TIMEOUT) });
await check("a call without a token, or with a forged one, is 401", async () => {
	const none = await raw("/api/notes");
	const forged = await raw("/api/notes", { headers: { authorization: `${bearer().slice(0, -4)}0000` } });
	const wrong = await raw("/api/oauth/token", { method: "POST", body: new URLSearchParams({ client_id: "id-1", client_secret: "nope" }) });
	const valid = await raw("/api/notes", { headers: { authorization: bearer() } });
	return [none.status === 401 && forged.status === 401 && wrong.status === 401 && valid.status === 200, { none: none.status, forged: forged.status, wrongSecret: wrong.status, valid: valid.status }];
});

await check("a websocket without a token is refused, and a message the channel refuses closes it (1008)", async () => {
	// The refusal is a 401. cf dev's server (the Vite plugin) drops the connection instead of passing it
	// on (an error, no status).
	// Upstream: cloudflare/workers-sdk#15170 (when fixed: expect only the 401)
	const refused = await new Promise<number | string>(resolve => {
		const socket = new RawWebSocket(`${wsBase}/api/notes/live`);
		socket.on("unexpected-response", (_, response) => { resolve(response.statusCode ?? "no status"); socket.terminate(); });
		socket.on("open", () => { resolve("opened"); socket.close(); });
		socket.on("error", error => resolve(String(error.message)));
	});
	// The feed's frames come first; then the refused message closes the socket.
	const closedBy = (message: unknown) => new Promise<number | string>(resolve => {
		const socket = new RawWebSocket(`${wsBase}/api/notes/live`, { headers: { authorization: bearer() } });
		socket.on("open", () => socket.send(JSON.stringify(message)));
		socket.on("close", code => resolve(code));
		socket.on("error", error => resolve(String(error.message)));
	});
	const notSubscribe = await closedBy({ subject: "not a subscribe message" });
	const otherTopic = await closedBy({ topic: "other" });
	return [(refused === 401 || (typeof refused === "string" && refused !== "opened")) && notSubscribe === 1008 && otherTopic === 1008, { refused, notSubscribe, otherTopic }];
});

console.log(failed ? `${failed} failed against ${base}` : `all passed against ${base} (SDK: sdk/out)`);
process.exit(failed ? 1 : 0);
