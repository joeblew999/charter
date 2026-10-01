// The generated TypeScript SDK (sdk/out/<sdk>/typescript-dist) against the live API, real-time paths
// only: notes.watch() (SSE) and liveNotes.connect() (WebSocket, NotesHub Durable Object) must both
// receive a note created with notes.create(). Usage: node sdk-live-test.mjs <origin> [sdk]
// (sdk: a folder in sdk/fern/apis, default api; api-go for the Go Worker)
const [origin, sdk = "api"] = process.argv.slice(2);
const { OrpcApiClient } = await import(`../sdk/out/${sdk}/typescript-dist/esm/index.mjs`);

const client = new OrpcApiClient({ baseUrl: origin });
const body = `sdk live ${Date.now()}`;
const got = { sse: null, ws: null };

const watching = (async () => {
  for await (const note of await client.notes.watch({ seconds: 20 })) if (note.body === body) { got.sse = note; return; }
})();
// One baseUrl serves HTTP and WebSocket: the socket needs the ws(s):// form.
const live = new OrpcApiClient({ baseUrl: origin.replace(/^http/, "ws") });
const socket = await live.liveNotes.connect({ reconnectAttempts: 0 });
const received = new Promise(resolve => socket.on("message", note => { if (note.body === body) { got.ws = note; resolve(); } }));
await socket.waitForOpen();
await new Promise(r => setTimeout(r, 1500)); // let the SSE stream attach to the hub

const created = await client.notes.create({ body });
await Promise.race([Promise.all([watching, received]), new Promise(r => setTimeout(r, 10000))]);
socket.close();

let failed = 0;
for (const [name, note] of Object.entries(got)) {
  const ok = note?.id === created.id;
  console.log(`${ok ? "PASS" : "FAIL"}  SDK ${name === "sse" ? "notes.watch() (SSE)" : "liveNotes.connect() (WebSocket, Durable Object)"}: ${JSON.stringify(note)}`);
  if (!ok) failed++;
}
process.exit(failed ? 1 : 0);
