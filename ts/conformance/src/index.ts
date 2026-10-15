// Does the Fern-generated TypeScript SDK (src/client, from fern) work on Cloudflare
// Workers? This Worker serves the conformance API itself under /api/mock/* (src/conformance.ts: the oRPC
// contract the SDK's specs are generated from, on D1 and the hub: src/hub.ts), and /api/sdk-test runs
// the SDK against it inside workerd, through the SDK's own `fetch` option (no network round trip).
import { ConformanceClient } from "./client/index.mjs";
import { WebhooksHelper } from "./client/webhooks/index.mjs";
import type { Report } from "./report.ts";
import { env } from "cloudflare:workers";
import { type Deployment, mockApi, seen, type Settings, trusted, webhookOf, wellKnownOf } from "./conformance.ts";
import { d1Notes } from "./hub.ts";
import { seeds } from "./notes.ts";
export { ConformanceHub } from "./hub.ts";

// The notes in D1 behind the hub; whom the API trusts, from the Worker's secrets; its Rate Limiting
// bindings; where its webhook goes. The four settings
// for Access and OIDC are optional secrets (cloudflare.config.ts), so the generated Env may not name them.
const settings = env as typeof env & Settings;
const deployment: Deployment = { notes: d1Notes(), trusted: trusted(settings), limits: env, webhook: webhookOf(settings) };

async function hmacHex(secret: string, body: string) {
	const key = await crypto.subtle.importKey("raw", new TextEncoder().encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
	const mac = await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(body));
	return [...new Uint8Array(mac)].map(b => b.toString(16).padStart(2, "0")).join("");
}

async function sdkTest(): Promise<Response> {
	let pages = 0;
	const client = new ConformanceClient({
		baseUrl: "https://mock.invalid/api/mock",
		oauth: { clientId: "id-1", clientSecret: "secret-1" },
		fetch: (input, init) => {
			const request = new Request(input, init);
			if (request.method === "GET" && new URL(request.url).pathname.endsWith("/notes")) pages++;
			return mockApi(request, deployment);
		},
	} as ConformanceClient.Options);
	const results: Report["results"] = {};
	const check = async (name: string, run: () => Promise<[boolean, unknown]>) => {
		try {
			const [ok, detail] = await run();
			results[name] = { ok, detail };
		} catch (error) {
			results[name] = { ok: false, detail: String(error) };
		}
	};

	// The log keeps what earlier runs created (D1): the run's notes have keys of their own, and the
	// checks read the seeded five at the log's start.
	await check("pagination (auto-paging: the seeded five over 3 pages)", async () => {
		const ids: string[] = [];
		for await (const note of await client.notes.list()) if (ids.push(note.id) === 5) break;
		return [ids.join() === "n1,n2,n3,n4,n5" && pages === 3, { ids, pages }];
	});
	await check("oauth client credentials (token fetched, form-encoded, reused)", async () => {
		return [seen.tokenCalls === 1 && seen.tokenContentType.includes("application/x-www-form-urlencoded"), { tokenCalls: seen.tokenCalls, contentType: seen.tokenContentType }];
	});
	await check("idempotent create (Idempotency-Key + bearer token sent; a retry gets the first answer)", async () => {
		const key = `key-${crypto.randomUUID().slice(0, 8)}`;
		const note = await client.notes.create({ body: "hello" }, { idempotencyKey: key });
		const again = await client.notes.create({ body: "something else" }, { idempotencyKey: key });
		return [note.id === key && again.id === key && again.body === "hello" && seen.authOnCreate === "Bearer tok-1", { note, again, authorization: seen.authOnCreate }];
	});
	await check("SSE stream (typed chunks)", async () => {
		const chunks: string[] = [];
		for await (const chunk of await client.chat({ prompt: "from workers" })) chunks.push(chunk.text);
		return [chunks.join(" ") === "echo from workers", chunks];
	});
	await check("resumable SSE (typed notes after a position, ended by the terminator)", async () => {
		const ids: string[] = [];
		const started = Date.now();
		for await (const note of await client.notes.watch({ after: "3", seconds: 1 })) ids.push(note.id);
		const ms = Date.now() - started;
		// The terminator ends it: a resumable stream without one would reconnect, and never end.
		return [ids.slice(0, 2).join() === "n4,n5" && ms < 5000, { ids: ids.slice(0, 5), ms }];
	});
	await check("file upload (multipart: the file and a field)", async () => {
		const stored = await client.files.uploadFile({ file: new File(["hello"], "hello.txt", { type: "text/plain" }), note: "greeting" });
		return [stored.id === "hello.txt:greeting" && stored.size === 5, stored];
	});
	await check("webhook HMAC signature (valid accepted, forged rejected)", async () => {
		const body = JSON.stringify({ event: "note.created", note: seeds[0] });
		const good = await WebhooksHelper.verifySignature(body, await hmacHex("whsec", body), "whsec");
		const bad = await WebhooksHelper.verifySignature(body, await hmacHex("wrong", body), "whsec");
		return [good && !bad, { good, bad }];
	});

	const passed = Object.values(results).every(r => r.ok);
	return Response.json({ passed, results } satisfies Report, { status: passed ? 200 : 500 });
}

async function wsTest(origin: string): Promise<Response> {
	const client = new ConformanceClient({
		baseUrl: `${origin.replace(/^http/, "ws")}/api/mock`,
		oauth: { clientId: "id-1", clientSecret: "secret-1" },
		fetch: (input, init) => mockApi(new Request(input, init), deployment),
	} as ConformanceClient.Options);
	try {
		// Workers can't send WebSocket headers, where the SDK puts the token: get it with the SDK's own
		// auth client and pass it as a query parameter (connect's standard queryParams option).
		const { access_token } = await client.auth.getToken({ client_id: "id-1", client_secret: "secret-1" });
		const socket = await client.liveNotes.connect({ reconnectAttempts: 0, connectionTimeoutInSeconds: 5, queryParams: { access_token } });
		const events: { event: string; id: string; auth?: string }[] = [];
		const done = new Promise<void>((resolve, reject) => {
			socket.on("message", (message: any) => { events.push(message); if (events.length === 3) resolve(); });
			socket.on("error", (error: any) => reject(new Error(String(error?.message || error?.error?.message || error?.type || error))));
			setTimeout(() => reject(new Error("no events within 5 s")), 5000);
		});
		await socket.waitForOpen();
		socket.sendSubscribe({ topic: "notes" });
		await done;
		socket.close();
		// The feed goes on past the seeded notes: the first three are the log's start.
		const gotEvents = events.slice(0, 3).map(e => e.id).join() === "n1,n2,n3";
		const gotAuth = events.every(e => e.auth === "Bearer tok-1");
		const results: Report["results"] = {
			"websocket client inside a Worker (typed events)": { ok: gotEvents, detail: events.map(e => e.id) },
			"websocket client inside a Worker (bearer token sent)": { ok: gotAuth, detail: events.map(e => e.auth ?? "") },
		};
		const ok = gotEvents && gotAuth;
		return Response.json({ passed: ok, results } satisfies Report, { status: ok ? 200 : 500 });
	} catch (error) {
		return Response.json({ passed: false, results: { "websocket client inside a Worker (typed events)": { ok: false, detail: String(error) } } } satisfies Report, { status: 500 });
	}
}

export default {
	async fetch(request) {
		const url = new URL(request.url);
		if (url.pathname === "/api/sdk-test") return sdkTest();
		// ?target= another Worker serving the mock API: on Cloudflare a Worker can't call its own URL (error 1042).
		if (url.pathname === "/api/ws-test") return wsTest(url.searchParams.get("target") ?? url.origin);
		if (url.pathname.startsWith("/api/mock/")) return mockApi(request, deployment);
		// What a client reads to find out how to get in: the metadata of the API at /api/mock.
		if (url.pathname.startsWith("/.well-known/")) return wellKnownOf(request, deployment) ?? new Response("not found", { status: 404 });
		return new Response("not found", { status: 404 });
	},
} satisfies ExportedHandler;
