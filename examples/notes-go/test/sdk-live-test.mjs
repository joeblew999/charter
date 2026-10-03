// The generated TypeScript SDK (sdk/out/typescript-dist) against the live API, real-time paths
// only: notes.watch() (SSE) and liveNotes.connect() (WebSocket, the hub Durable Object) must both
// receive a note created with notes.create(). Usage, from the project's folder (the SDK is the one
// in its sdk/out): node test/sdk-live-test.mjs <origin>
import { pathToFileURL } from "node:url";
const origin = process.argv[2];
const { NotesClient } = await import(pathToFileURL(`${process.cwd()}/sdk/out/typescript-dist/esm/index.mjs`));

// Writing needs WRITE_TOKEN (charter exec -secrets sets it); the SDK sends it as a bearer token.
const client = new NotesClient({ baseUrl: origin, token: process.env.WRITE_TOKEN });
const body = `sdk live ${Date.now()}`;
const got = { sse: null, ws: null };

const watching = (async () => {
  for await (const note of await client.notes.watch({ seconds: 20 })) if (note.body === body) { got.sse = note; return; }
})();
// One baseUrl serves HTTP and WebSocket: the socket needs the ws(s):// form.
const live = new NotesClient({ baseUrl: origin.replace(/^http/, "ws") });
const socket = await live.liveNotes.connect({ reconnectAttempts: 0 });
const received = new Promise(resolve => socket.on("message", note => { if (note.body === body) { got.ws = note; resolve(); } }));
await socket.waitForOpen();
await new Promise(r => setTimeout(r, 1500)); // let the SSE stream attach to the hub

const created = await client.notes.create({ body });
await Promise.race([Promise.all([watching, received]), new Promise(r => setTimeout(r, 10000))]);
socket.close();

let failed = 0;
for (const [name, note] of Object.entries(got)) {
  const ok = Number.isInteger(created?.id) && note?.id === created.id;
  console.log(`${ok ? "PASS" : "FAIL"}  SDK ${name === "sse" ? "notes.watch() (SSE)" : "liveNotes.connect() (WebSocket, Durable Object)"}: ${JSON.stringify(note)}`);
  if (!ok) failed++;
}
process.exit(failed ? 1 : 0);
