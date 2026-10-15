// The generated TypeScript SDK (sdk/out/typescript-dist, npm run sdk) against the live API, real-time
// paths only: notes.watch() (SSE) and liveNotes.connect() (WebSocket, the hub Durable Object) must
// both receive a note created with notes.create(), each checked against the contract's schema.
// Usage, from the project's folder (the SDK is the one in its sdk/out): node test/sdk.ts <origin>
import { pathToFileURL } from "node:url";
import type { z } from "zod";
import { note } from "../api/schemas.ts";
import { accessHeaders } from "./access.ts";

type Note = z.infer<typeof note>;
// What this test uses of the SDK. The SDK is generated after the tests are typechecked, so this says
// it here; the notes it returns are checked against the contract when they arrive.
type Client = {
	notes: { watch(request: { seconds: number }): Promise<AsyncIterable<unknown>>; create(request: { body: string }): Promise<unknown> };
	liveNotes: { connect(options: { reconnectAttempts: number }): Promise<{ on(event: "message", listener: (message: unknown) => void): void; waitForOpen(): Promise<unknown>; close(): void }> };
};
type ClientOptions = { baseUrl: string; headers: Record<string, string>; bearer?: { token: string }; auth?: false };

const origin = process.argv[2], token = process.env.WRITE_TOKEN;
if (!origin || !token) {
	console.log("FAIL  usage: node test/sdk.ts <origin>, with WRITE_TOKEN set (fnox exec --)");
	process.exit(1);
}
const { NotesClient } = (await import(pathToFileURL(`${process.cwd()}/sdk/out/typescript-dist/esm/index.mjs`).href)) as { NotesClient: new (options: ClientOptions) => Client };

// Writing needs WRITE_TOKEN; the SDK sends it as a bearer token. Behind Cloudflare Access, every
// request also carries the service token, as headers: the SDK would send only one of the two by itself.
// Upstream: fern-api/fern#17775 (when fixed: the SDK reads both from <WORKER>_ACCESS_CLIENT_ID and _SECRET)
const headers = accessHeaders(origin);
const client = new NotesClient({ baseUrl: origin, bearer: { token }, headers });
const body = `sdk live ${Date.now()}`;
const got: Record<"sse" | "ws", Note | null> = { sse: null, ws: null };

const watching = (async () => {
	for await (const message of await client.notes.watch({ seconds: 20 })) {
		const received = note.parse(message);
		if (received.body === body) { got.sse = received; return; }
	}
})();
// One baseUrl serves HTTP and WebSocket: the socket needs the ws(s):// form. The channel is public, but
// the SDK asks for credentials for it: auth: false says there are none.
const live = new NotesClient({ baseUrl: origin.replace(/^http/, "ws"), auth: false, headers });
const socket = await live.liveNotes.connect({ reconnectAttempts: 0 });
const received = new Promise<void>(resolve => socket.on("message", message => {
	const n = note.parse(message);
	if (n.body === body) { got.ws = n; resolve(); }
}));
await socket.waitForOpen();
await new Promise(resolve => setTimeout(resolve, 1500)); // let the SSE stream attach to the hub

const created = note.parse(await client.notes.create({ body }));
await Promise.race([Promise.all([watching, received]), new Promise(resolve => setTimeout(resolve, 10000))]);
socket.close();

let failed = 0;
for (const name of ["sse", "ws"] as const) {
	const ok = got[name]?.id === created.id;
	console.log(`${ok ? "PASS" : "FAIL"}  SDK ${name === "sse" ? "notes.watch() (SSE)" : "liveNotes.connect() (WebSocket, Durable Object)"}: ${JSON.stringify(got[name])}`);
	if (!ok) failed++;
}
process.exit(failed ? 1 : 0);
