// Live test of the API's real-time paths: an SSE client (GET /api/notes/watch) and a WebSocket
// client (/api/notes/live) connect, a note is created, and both must receive it (via oRPC's
// publisher on the NotesHub Durable Object); then SSE resume after a reconnect. Usage: node live-test.mjs <origin>   (uses `ws` from sdk/node_modules)
import { createRequire } from "node:module";
const WebSocket = createRequire(new URL("../sdk/package.json", import.meta.url))("ws");

const origin = process.argv[2];
const body = `live ${Date.now()}`;
const got = { sse: null, ws: null, resume: null };

// SSE: read the event stream until our note arrives.
const sse = (async () => {
  const res = await fetch(`${origin}/api/notes/watch?seconds=20`, { headers: { accept: "text/event-stream" } });
  const decoder = new TextDecoder(); let buf = "";
  for await (const chunk of res.body) {
    buf += decoder.decode(chunk, { stream: true });
    for (const line of buf.split("\n")) if (line.startsWith("data:") && line.includes(body)) { got.sse = JSON.parse(line.slice(5)); return; }
  }
})();

// WebSocket: wait for our note.
const ws = new WebSocket(`${origin.replace(/^http/, "ws")}/api/notes/live`);
const wsDone = new Promise((resolve, reject) => {
  ws.on("message", data => { const note = JSON.parse(String(data)); if (note.body === body) { got.ws = note; resolve(); } });
  ws.on("error", reject);
});
await new Promise(r => ws.on("open", r));
await new Promise(r => setTimeout(r, 1500)); // let the SSE stream attach to the hub

const created = await (await fetch(`${origin}/api/notes`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ body }) })).json();
await Promise.race([Promise.all([sse, wsDone]), new Promise(r => setTimeout(r, 10000))]);
ws.close();

// Resume: disconnect, miss a note, reconnect with Last-Event-ID -> the missed note is replayed
// from D1, the log: follow() catches up from the position the client names.
async function sseEvents(headers, seconds, until) {
  const res = await fetch(`${origin}/api/notes/watch?seconds=${seconds}`, { headers: { accept: "text/event-stream", ...headers } });
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
await fetch(`${origin}/api/notes`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ body: `${body} a` }) });
const lastId = (await first).at(-1)?.id;
const missed = await (await fetch(`${origin}/api/notes`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ body: `${body} b` }) })).json();
const replayed = await sseEvents({ "last-event-id": lastId }, 4, events => events.some(e => e.note.id === missed.id));
got.resume = replayed.find(e => e.note.id === missed.id)?.note ?? null;
created.resumeId = missed.id;

let failed = 0;
for (const [name, note] of Object.entries(got)) {
  const ok = note?.id === (name === "resume" ? created.resumeId : created.id);
  const label = { sse: "SSE /api/notes/watch", ws: "WebSocket /api/notes/live (Durable Object)", resume: "SSE resume: missed note replayed after reconnect (Last-Event-ID)" }[name];
  console.log(`${ok ? "PASS" : "FAIL"}  ${label}: ${JSON.stringify(note)}`);
  if (!ok) failed++;
}
process.exit(failed ? 1 : 0);
