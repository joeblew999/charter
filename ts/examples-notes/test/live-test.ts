// Live test of the API's real-time paths: an SSE client (GET /api/notes/watch) and a WebSocket
// client (/api/notes/live) connect, a note is created, and both must receive it (through
// the hub Durable Object); then SSE resume after a reconnect. Every note the API sends is checked
// against the contract's schema. Usage: node test/live-test.ts <origin>. Writing needs a token:
// WRITE_TOKEN; READ_TOKEN must be refused (fnox exec -- sets both). Behind Cloudflare Access, every
// request also carries the machine's service token (test/access.ts), and what Access and the Worker
// refuse is checked too.
import WebSocket from "ws";
import type { z } from "zod";
import { END, note } from "../src/contract.ts";
import { accessHeaders } from "./access.ts";

type Note = z.infer<typeof note>;
/** One SSE event: its id, which a client resumes from, and the note. */
type Event = { id: string | undefined; note: Note };

const origin = process.argv[2];
const { WRITE_TOKEN, READ_TOKEN } = process.env;
if (!origin || !WRITE_TOKEN || !READ_TOKEN) {
	console.log("FAIL  usage: node test/live-test.ts <origin>, with WRITE_TOKEN and READ_TOKEN set (fnox exec --)");
	process.exit(1);
}
const edge = accessHeaders(origin);
const writer = { "content-type": "application/json", ...edge, authorization: `Bearer ${WRITE_TOKEN}` };
const body = `live ${Date.now()}`;
const create = async (text: string): Promise<Note> => note.parse(await (await fetch(`${origin}/api/notes`, { method: "POST", headers: writer, body: JSON.stringify({ body: text }) })).json());

/** Read the notes stream until `until` says enough, it ends (END), or `seconds` run out. */
async function sseEvents(headers: Record<string, string>, seconds: number, until: (events: Event[]) => boolean): Promise<Event[]> {
	const res = await fetch(`${origin}/api/notes/watch?seconds=${seconds}`, { headers: { accept: "text/event-stream", ...edge, ...headers } });
	if (!res.body) throw new Error(`GET /api/notes/watch: ${res.status}, no stream`);
	const decoder = new TextDecoder(), events: Event[] = [];
	let buf = "";
	for await (const chunk of res.body) {
		buf += decoder.decode(chunk, { stream: true });
		const blocks = buf.split("\n\n");
		buf = blocks.pop() ?? "";
		for (const block of blocks) {
			const id = /^id: ?(.*)$/m.exec(block)?.[1], data = /^data: ?(.*)$/m.exec(block)?.[1];
			if (!data) continue;
			// The planned end of the stream is END, sent as JSON like every item: "[end-of-stream]".
			const value: unknown = JSON.parse(data);
			if (value === END) return events;
			events.push({ id, note: note.parse(value) });
			if (until(events)) return events;
		}
	}
	return events;
}

const got: Record<"sse" | "ws" | "resume", Note | null> = { sse: null, ws: null, resume: null };

// SSE and WebSocket both wait for our note.
const sse = sseEvents({}, 20, events => events.some(e => e.note.body === body)).then(events => { got.sse = events.find(e => e.note.body === body)?.note ?? null; });
const ws = new WebSocket(`${origin.replace(/^http/, "ws")}/api/notes/live`, { headers: edge });
const wsDone = new Promise<void>((resolve, reject) => {
	ws.on("message", data => {
		const received = note.parse(JSON.parse(String(data)));
		if (received.body === body) { got.ws = received; resolve(); }
	});
	ws.on("error", reject);
});
await new Promise<void>(resolve => ws.once("open", () => resolve()));
await new Promise(resolve => setTimeout(resolve, 1500)); // let the SSE stream attach to the hub

const created = await create(body);
await Promise.race([Promise.all([sse, wsDone]), new Promise(resolve => setTimeout(resolve, 10000))]);
ws.close();

// Resume: disconnect, miss a note, reconnect with Last-Event-ID -> the missed note is replayed
// from D1, the log: follow() catches up from the position the client names.
const first = sseEvents({}, 8, events => events.some(e => e.note.body === `${body} a`));
await new Promise(resolve => setTimeout(resolve, 1500));
await create(`${body} a`);
const lastId = (await first).at(-1)?.id ?? "";
const missed = await create(`${body} b`);
const replayed = await sseEvents({ "last-event-id": lastId }, 4, events => events.some(e => e.note.id === missed.id));
got.resume = replayed.find(e => e.note.id === missed.id)?.note ?? null;

// Who may write. Without Access: no token is 401, the read token 403, a JWT from an issuer the
// Worker does not trust 401. Behind Access: Access refuses a request without the service token or
// with a wrong secret (401 or a redirect to the login: never the Worker); the machine's token alone
// reads but may not write (403), nor with the read token (403); with an untrusted JWT, 401.
const post = async (headers: Record<string, string>): Promise<number> => (await fetch(`${origin}/api/notes`, { method: "POST", headers: { "content-type": "application/json", ...headers }, body: JSON.stringify({ body }), redirect: "manual" })).status;
const untrusted = `Bearer ${Buffer.from('{"alg":"RS256","kid":"x"}').toString("base64url")}.${Buffer.from('{"iss":"https://untrusted.example","sub":"x"}').toString("base64url")}.c2ln`;
const refused: Record<string, [status: number, want: number | "401 or 302"]> = edge["cf-access-client-id"] ? {
	"no Access credentials (Access refuses it)": [await post({}), "401 or 302"],
	"a wrong service token secret (Access refuses it)": [await post({ ...edge, "cf-access-client-secret": "wrong" }), "401 or 302"],
	"the machine's service token alone": [await post(edge), 403],
	"the machine's service token and the read token": [await post({ ...edge, authorization: `Bearer ${READ_TOKEN}` }), 403],
	"the machine's service token and a JWT from an untrusted issuer": [await post({ ...edge, authorization: untrusted }), 401],
} : {
	"no token": [await post({}), 401],
	"the read token": [await post({ authorization: `Bearer ${READ_TOKEN}` }), 403],
	"a JWT from an untrusted issuer": [await post({ authorization: untrusted }), 401],
};

let failed = 0;
for (const [name, [status, want]] of Object.entries(refused)) {
	const ok = want === "401 or 302" ? status === 401 || status === 302 : status === want;
	console.log(`${ok ? "PASS" : "FAIL"}  POST /api/notes with ${name}: ${status} (want ${want})`);
	if (!ok) failed++;
}
const checks = {
	sse: ["SSE /api/notes/watch", created.id],
	ws: ["WebSocket /api/notes/live (Durable Object)", created.id],
	resume: ["SSE resume: missed note replayed after reconnect (Last-Event-ID)", missed.id],
} as const;
for (const name of ["sse", "ws", "resume"] as const) {
	const [label, want] = checks[name];
	const ok = got[name]?.id === want;
	console.log(`${ok ? "PASS" : "FAIL"}  ${label}: ${JSON.stringify(got[name])}`);
	if (!ok) failed++;
}
process.exit(failed ? 1 : 0);
