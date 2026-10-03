// Live test of the API's real-time paths: an SSE client (GET /api/notes/watch) and a WebSocket
// client (/api/notes/live) connect, a note is created, and both must receive it (through
// the hub Durable Object); then SSE resume after a reconnect. Usage, from the project's
// folder (`ws` comes from its node_modules): node test/live-test.mjs <origin>. Writing needs a
// token: WRITE_TOKEN; READ_TOKEN must be refused (charter exec -secrets sets both). Behind
// Cloudflare Access, every request also carries the machine's service token (test/access.mjs), and
// what Access and the Worker refuse is checked too.
import { createRequire } from "node:module";
import { accessHeaders } from "./access.mjs";
const WebSocket = createRequire(`${process.cwd()}/`)("ws");

const origin = process.argv[2];
const { WRITE_TOKEN, READ_TOKEN } = process.env;
if (!WRITE_TOKEN || !READ_TOKEN) { console.log("FAIL  WRITE_TOKEN and READ_TOKEN are not set (run it through charter exec -secrets)"); process.exit(1); }
const edge = accessHeaders(process.argv[2]);
const json = { "content-type": "application/json", ...edge };
const writer = { ...json, authorization: `Bearer ${WRITE_TOKEN}` };
const body = `live ${Date.now()}`;
const got = { sse: null, ws: null, resume: null };

// SSE: read the event stream until our note arrives.
const sse = (async () => {
  const res = await fetch(`${origin}/api/notes/watch?seconds=20`, { headers: { accept: "text/event-stream", ...edge } });
  const decoder = new TextDecoder(); let buf = "";
  for await (const chunk of res.body) {
    buf += decoder.decode(chunk, { stream: true });
    for (const line of buf.split("\n")) if (line.startsWith("data:") && line.includes(body)) { got.sse = JSON.parse(line.slice(5)); return; }
  }
})();

// WebSocket: wait for our note.
const ws = new WebSocket(`${origin.replace(/^http/, "ws")}/api/notes/live`, { headers: edge });
const wsDone = new Promise((resolve, reject) => {
  ws.on("message", data => { const note = JSON.parse(String(data)); if (note.body === body) { got.ws = note; resolve(); } });
  ws.on("error", reject);
});
await new Promise(r => ws.on("open", r));
await new Promise(r => setTimeout(r, 1500)); // let the SSE stream attach to the hub

const created = await (await fetch(`${origin}/api/notes`, { method: "POST", headers: writer, body: JSON.stringify({ body }) })).json();
await Promise.race([Promise.all([sse, wsDone]), new Promise(r => setTimeout(r, 10000))]);
ws.close();

// Resume: disconnect, miss a note, reconnect with Last-Event-ID -> the missed note is replayed
// from D1, the log: follow() catches up from the position the client names.
async function sseEvents(headers, seconds, until) {
  const res = await fetch(`${origin}/api/notes/watch?seconds=${seconds}`, { headers: { accept: "text/event-stream", ...edge, ...headers } });
  const decoder = new TextDecoder(); let buf = ""; const events = [];
  for await (const chunk of res.body) {
    buf += decoder.decode(chunk, { stream: true });
    const blocks = buf.split("\n\n"); buf = blocks.pop();
    for (const block of blocks) {
      const id = /^id: ?(.*)$/m.exec(block)?.[1], data = /^data: ?(.*)$/m.exec(block)?.[1];
      if (data) { events.push({ id, note: JSON.parse(data) }); if (until(events)) return events; }
    }
  }
  return events;
}
const first = sseEvents({}, 8, events => events.some(e => e.note.body === `${body} a`));
await new Promise(r => setTimeout(r, 1500));
await fetch(`${origin}/api/notes`, { method: "POST", headers: writer, body: JSON.stringify({ body: `${body} a` }) });
const lastId = (await first).at(-1)?.id;
const missed = await (await fetch(`${origin}/api/notes`, { method: "POST", headers: writer, body: JSON.stringify({ body: `${body} b` }) })).json();
const replayed = await sseEvents({ "last-event-id": lastId }, 4, events => events.some(e => e.note.id === missed.id));
got.resume = replayed.find(e => e.note.id === missed.id)?.note ?? null;
created.resumeId = missed.id;

// Who may write. Without Access: no token is 401, the read token 403, a JWT from an issuer the
// Worker does not trust 401. Behind Access: Access refuses a request without the service token or
// with a wrong secret (401 or a redirect to the login: never the Worker); the machine's token alone
// reads but may not write (403), nor with the read token (403); with an untrusted JWT, 401.
const post = async headers => (await fetch(`${origin}/api/notes`, { method: "POST", headers: { "content-type": "application/json", ...headers }, body: JSON.stringify({ body }), redirect: "manual" })).status;
const untrusted = `Bearer ${Buffer.from('{"alg":"RS256","kid":"x"}').toString("base64url")}.${Buffer.from('{"iss":"https://untrusted.example","sub":"x"}').toString("base64url")}.c2ln`;
const refused = edge["cf-access-client-id"] ? {
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
for (const [name, note] of Object.entries(got)) {
  const want = name === "resume" ? created.resumeId : created.id;
  const ok = Number.isInteger(want) && note?.id === want;
  const label = { sse: "SSE /api/notes/watch", ws: "WebSocket /api/notes/live (Durable Object)", resume: "SSE resume: missed note replayed after reconnect (Last-Event-ID)" }[name];
  console.log(`${ok ? "PASS" : "FAIL"}  ${label}: ${JSON.stringify(note)}`);
  if (!ok) failed++;
}
process.exit(failed ? 1 : 0);
