// Does the Fern-generated TypeScript SDK (src/client, from sdk/fern/apis/showcase) work on Cloudflare
// Workers? This Worker serves the showcase API itself under /api/mock/* (src/showcase.ts: the oRPC
// contract the SDK's specs are generated from), and /api/sdk-test runs the SDK against it inside
// workerd, through the SDK's own `fetch` option (no network round trip).
import { ShowcaseClient } from "./client/index.mjs";
import { WebhooksHelper } from "./client/webhooks/index.mjs";
import { mockApi, notes, seen } from "./showcase.ts";

async function hmacHex(secret: string, body: string) {
	const key = await crypto.subtle.importKey("raw", new TextEncoder().encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
	const mac = await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(body));
	return [...new Uint8Array(mac)].map(b => b.toString(16).padStart(2, "0")).join("");
}

async function sdkTest(): Promise<Response> {
	const client = new ShowcaseClient({
		baseUrl: "https://mock.invalid/api/mock",
		clientId: "id-1",
		clientSecret: "secret-1",
		fetch: (input, init) => mockApi(new Request(input, init)),
	} as ShowcaseClient.Options);
	const results: Record<string, { ok: boolean; detail: unknown }> = {};
	const check = async (name: string, run: () => Promise<[boolean, unknown]>) => {
		try {
			const [ok, detail] = await run();
			results[name] = { ok, detail };
		} catch (error) {
			results[name] = { ok: false, detail: String(error) };
		}
	};

	await check("pagination (auto-paging over 3 pages)", async () => {
		const ids: string[] = [];
		for await (const note of await client.notes.list()) ids.push(note.id);
		return [ids.join() === "n1,n2,n3,n4,n5", ids];
	});
	await check("oauth client credentials (token fetched, form-encoded, reused)", async () => {
		return [seen.tokenCalls === 1 && seen.tokenContentType.includes("application/x-www-form-urlencoded"), { tokenCalls: seen.tokenCalls, contentType: seen.tokenContentType }];
	});
	await check("idempotent create (Idempotency-Key + bearer token sent)", async () => {
		const note = await client.notes.create({ body: "hello" }, { idempotencyKey: "key-123" });
		return [note.id === "key-123" && seen.authOnCreate === "Bearer tok-1", { note, authorization: seen.authOnCreate }];
	});
	await check("SSE stream (typed chunks)", async () => {
		const chunks: string[] = [];
		for await (const chunk of await client.chat({ prompt: "from workers" })) chunks.push(chunk.text);
		return [chunks.join(" ") === "echo from workers", chunks];
	});
	await check("webhook HMAC signature (valid accepted, forged rejected)", async () => {
		const body = JSON.stringify({ event: "note.created", note: notes[0] });
		const good = await WebhooksHelper.verifySignature(body, await hmacHex("whsec", body), "whsec");
		const bad = await WebhooksHelper.verifySignature(body, await hmacHex("wrong", body), "whsec");
		return [good && !bad, { good, bad }];
	});

	const passed = Object.values(results).every(r => r.ok);
	return Response.json({ passed, results }, { status: passed ? 200 : 500 });
}

async function wsTest(origin: string): Promise<Response> {
	const client = new ShowcaseClient({
		baseUrl: `${origin.replace(/^http/, "ws")}/api/mock`,
		clientId: "id-1",
		clientSecret: "secret-1",
		fetch: (input, init) => mockApi(new Request(input, init)),
	} as ShowcaseClient.Options);
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
		const gotEvents = events.map(e => e.id).join() === "n1,n2,n3";
		const gotAuth = events.every(e => e.auth === "Bearer tok-1");
		const results = {
			"websocket client inside a Worker (typed events)": { ok: gotEvents, detail: events.map(e => e.id) },
			"websocket client inside a Worker (bearer token sent)": { ok: gotAuth, detail: events.map(e => e.auth ?? "") },
		};
		const ok = gotEvents && gotAuth;
		return Response.json({ passed: ok, results }, { status: ok ? 200 : 500 });
	} catch (error) {
		return Response.json({ passed: false, results: { "websocket client inside a Worker (typed events)": { ok: false, detail: String(error) } } }, { status: 500 });
	}
}

export default {
	async fetch(request) {
		const url = new URL(request.url);
		if (url.pathname === "/api/sdk-test") return sdkTest();
		// ?target= another Worker serving the mock API: on Cloudflare a Worker can't call its own URL (error 1042).
		if (url.pathname === "/api/ws-test") return wsTest(url.searchParams.get("target") ?? url.origin);
		if (url.pathname.startsWith("/api/mock/")) return mockApi(request);
		return new Response("not found", { status: 404 });
	},
} satisfies ExportedHandler;
