// The generated SDK's WebSocket client from Node (it uses the `ws` package there), against the
// Worker's liveNotes channel over the real network. Usage: node ws-client.mjs <worker origin>
import { ShowcaseClient } from "../out/showcase-ts/typescript-dist/esm/index.mjs";

const origin = process.argv[2];
const client = new ShowcaseClient({
  baseUrl: `${origin.replace(/^http/, "ws")}/api/mock`,
  clientId: "id-1",
  clientSecret: "secret-1",
  // One baseUrl serves REST and WebSocket: send the OAuth token call over HTTP(S).
  fetch: (input, init) => fetch(String(input).replace(/^ws/, "http"), init),
});
const events = [];
const socket = await client.liveNotes.connect({ reconnectAttempts: 0, connectionTimeoutInSeconds: 10 });
const done = new Promise((resolve, reject) => {
  socket.on("message", message => { events.push(message); if (events.length === 3) resolve(); });
  socket.on("error", error => reject(error));
  setTimeout(() => reject(new Error("no events within 10 s")), 10000);
});
await socket.waitForOpen();
socket.sendSubscribe({ topic: "notes" });
await done;
socket.close();
const ok = events.map(e => e.id).join() === "n1,n2,n3" && events.every(e => e.auth === "Bearer tok-1");
console.log(`${ok ? "PASS" : "FAIL"}  websocket client from Node (typed events, bearer token sent): ${JSON.stringify(events)}`);
process.exit(ok ? 0 : 1);
