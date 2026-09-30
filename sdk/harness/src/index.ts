// Does the Fern-generated TypeScript SDK (src/client, from sdk/fern/apis/showcase) work on Cloudflare
// Workers? This Worker implements the showcase API itself under /api/mock/*, and /api/sdk-test runs the
// SDK against it inside workerd, through the SDK's own `fetch` option (no network round trip).
import { ShowcaseClient } from "./client/index.mjs";
import { WebhooksHelper } from "./client/webhooks/index.mjs";

const notes = ["n1", "n2", "n3", "n4", "n5"].map(id => ({ id, body: `note ${id}` }));
const seen = { tokenCalls: 0, tokenContentType: "", authOnCreate: "", idempotencyKey: "" };

async function mockApi(request: Request): Promise<Response> {
	const url = new URL(request.url);
	const route = `${request.method} ${url.pathname.replace(/^\/api\/mock/, "")}`;
	switch (route) {
		case "POST /oauth/token": {
			seen.tokenCalls++;
			seen.tokenContentType = request.headers.get("content-type") ?? "";
			const form = new URLSearchParams(await request.text());
			if (form.get("client_id") !== "id-1" || form.get("client_secret") !== "secret-1") return new Response("bad client", { status: 401 });
			return Response.json({ access_token: "tok-1", expires_in: 3600 });
		}
		case "GET /notes": {
			const start = Number(url.searchParams.get("cursor") ?? 0);
			const next = start + 2;
			return Response.json({ data: notes.slice(start, next), next_cursor: next < notes.length ? String(next) : undefined });
		}
		case "POST /notes": {
			seen.authOnCreate = request.headers.get("authorization") ?? "";
			seen.idempotencyKey = request.headers.get("idempotency-key") ?? "";
			const { body } = await request.json<{ body: string }>();
			return Response.json({ id: seen.idempotencyKey || "new", body });
		}
		case "POST /files": {
			const form = await request.formData();
			const file = form.get("file");
			if (!(file instanceof File)) return new Response("missing file", { status: 400 });
			return Response.json({ id: `${file.name}:${form.get("note") ?? ""}`, size: file.size });
		}
		case "GET /notes/live": {
			if (request.headers.get("upgrade") !== "websocket") return new Response("expected a WebSocket upgrade", { status: 426 });
			const [client, server] = Object.values(new WebSocketPair());
			// The SDK sends the token as a header (Node). Workers and browsers can't set WebSocket headers,
			// so it may come as ?access_token= instead.
			const token = url.searchParams.get("access_token");
			const auth = request.headers.get("authorization") ?? (token ? `Bearer ${token}` : "");
			server.accept();
			server.addEventListener("message", event => {
				const { topic } = JSON.parse(String(event.data)) as { topic: string };
				for (const note of notes.slice(0, 3)) server.send(JSON.stringify({ event: `${topic}.created`, id: note.id, body: note.body, auth }));
			});
			return new Response(null, { status: 101, webSocket: client });
		}
		case "POST /chat": {
			const { prompt } = await request.json<{ prompt: string }>();
			const words = `echo ${prompt}`.split(" ");
			const stream = new ReadableStream({
				async start(controller) {
					const enc = new TextEncoder();
					for (const [i, text] of words.entries()) {
						controller.enqueue(enc.encode(`data: ${JSON.stringify({ text, done: i === words.length - 1 })}\n\n`));
						await new Promise(r => setTimeout(r, 10));
					}
					controller.close();
				},
			});
			return new Response(stream, { headers: { "content-type": "text/event-stream" } });
		}
	}
	return new Response("not found", { status: 404 });
}

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
