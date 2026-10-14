// Test of a showcase API server with the TypeScript SDK Fern generated from its specs: every Fern
// feature the showcase has must work over the real network. It runs against the Go server
// (this project, natively or as Wasm under workerd) with the SDK from the Go contract's specs, and
// against the oRPC one (../showcase-ts, <origin>/api/mock) with the SDK from the oRPC contract's.
//
// Usage, from the project's folder (the SDK is the one in its sdk/out, `ws` from its node_modules):
//   node <this file> <base url> [--webhook-port <port>] [--open]
//   --webhook-port  listen there for the noteCreated webhook: start the server with
//                   WEBHOOK_URL=http://localhost:<port>/webhook
//   --open          the server checks no tokens (the oRPC showcase): leave out the checks that it does
import { existsSync } from "node:fs";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import { pathToFileURL } from "node:url";

const args = process.argv.slice(2);
const flag = name => (args.includes(name) ? args.splice(args.indexOf(name), 1).length > 0 : false);
const option = name => (args.includes(name) ? args.splice(args.indexOf(name), 2)[1] : undefined);
const open = flag("--open");
const webhookPort = option("--webhook-port");
const [base] = args;
const out = pathToFileURL(`${process.cwd()}/sdk/out/`);
const { ShowcaseClient } = await import(new URL("typescript-dist/esm/index.mjs", out));
const { WebhooksHelper } = await import(new URL("typescript-dist/esm/webhooks/index.mjs", out));
const RawWebSocket = createRequire(`${process.cwd()}/`)("ws");

const TIMEOUT = 15000;
const credentials = { clientId: "id-1", clientSecret: "secret-1" };
const webhookSecret = "whsec";
const wsBase = base.replace(/^http/, "ws");

let failed = 0;
async function check(name, run) {
  let ok = false, detail;
  try {
    [ok, detail] = await Promise.race([run(), new Promise((_, reject) => setTimeout(() => reject(new Error(`nothing within ${TIMEOUT / 1000} s`)), TIMEOUT))]);
  } catch (error) {
    detail = String(error?.message ?? error);
  }
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok ? "" : `: ${typeof detail === "string" ? detail : JSON.stringify(detail)}`}`);
  if (!ok) failed++;
}

// Everything the SDK sends goes through here, so the test sees it as the server does.
const sent = [];
const recordingFetch = (input, init = {}) => {
  const request = new Request(input, init);
  sent.push({ method: request.method, path: new URL(request.url).pathname.slice(new URL(base).pathname.replace(/\/$/, "").length), headers: request.headers });
  return fetch(input, { ...init, signal: init.signal ?? AbortSignal.timeout(TIMEOUT) });
};
const requests = (method, path) => sent.filter(request => request.method === method && request.path === path);
// One environment: `base` is the REST API, `production` (the AsyncAPI server's name) the WebSocket.
const client = new ShowcaseClient({ environment: { base, production: wsBase }, ...credentials, fetch: recordingFetch });
const bearer = () => requests("POST", "/notes").at(-1)?.headers.get("authorization") ?? "";

// The webhook receiver: what a customer of the API runs.
const deliveries = [];
const receiver = webhookPort && createServer(async (request, response) => {
  let body = "";
  for await (const chunk of request) body += chunk;
  deliveries.push({ body, signature: request.headers["x-webhook-signature"] ?? "" });
  response.end("ok");
}).listen(Number(webhookPort));

await check("pagination (auto-paging over 3 pages)", async () => {
  const ids = [];
  for await (const note of await client.notes.list()) ids.push(note.id);
  const pages = requests("GET", "/notes").length;
  return [ids.join() === "n1,n2,n3,n4,n5" && pages === 3, { ids, pages }];
});

await check("idempotent create (Idempotency-Key + bearer token sent)", async () => {
  const note = await client.notes.create({ body: "hello" }, { idempotencyKey: "key-123" });
  const again = await client.notes.create({ body: "hello" }, { idempotencyKey: "key-123" });
  const request = requests("POST", "/notes").at(-1);
  const ok = note.id === "key-123" && again.id === note.id && request.headers.get("idempotency-key") === "key-123" && bearer().startsWith("Bearer ");
  return [ok, { note, again, authorization: bearer() }];
});

await check("oauth client credentials (token fetched, form-encoded, reused)", async () => {
  const tokens = requests("POST", "/oauth/token");
  const contentType = tokens[0]?.headers.get("content-type") ?? "";
  const authorized = sent.filter(request => request.path !== "/oauth/token").every(request => request.headers.get("authorization") === bearer());
  return [tokens.length === 1 && contentType.includes("application/x-www-form-urlencoded") && authorized, { tokenCalls: tokens.length, contentType, authorized }];
});

await check("SSE stream (typed chunks)", async () => {
  const chunks = [];
  for await (const chunk of await client.chat({ prompt: "from the sdk" })) chunks.push(chunk);
  const text = chunks.map(chunk => chunk.text).join(" ");
  return [text === "echo from the sdk" && chunks.at(-1)?.done === true, chunks];
});

await check("multipart file upload", async () => {
  const stored = await client.files.uploadFile({ file: new File(["x".repeat(20000)], "hello.txt", { type: "text/plain" }), note: "a note" });
  return [stored.id === "hello.txt:a note" && stored.size === 20000, stored];
});

await check("webhook HMAC signature: the SDK's helper (valid accepted, forged rejected)", async () => {
  const body = JSON.stringify({ event: "note.created", note: { id: "n1", body: "note n1" } });
  const hex = async secret => {
    const key = await crypto.subtle.importKey("raw", new TextEncoder().encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
    return Buffer.from(await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(body))).toString("hex");
  };
  const good = await WebhooksHelper.verifySignature(body, await hex(webhookSecret), webhookSecret);
  const bad = await WebhooksHelper.verifySignature(body, await hex("wrong"), webhookSecret);
  return [good && !bad, { good, bad }];
});

if (receiver) {
  await check("webhook delivered and signed by the server (the SDK's helper verifies it)", async () => {
    for (let waited = 0; deliveries.length < 2 && waited < 5000; waited += 100) await new Promise(resolve => setTimeout(resolve, 100));
    const delivery = deliveries[0];
    if (!delivery) return [false, "no delivery: is the server's WEBHOOK_URL this receiver?"];
    const payload = JSON.parse(delivery.body);
    const good = await WebhooksHelper.verifySignature(delivery.body, delivery.signature, webhookSecret);
    const bad = await WebhooksHelper.verifySignature(delivery.body, delivery.signature, "wrong");
    const tampered = await WebhooksHelper.verifySignature(delivery.body.replace("hello", "hullo"), delivery.signature, webhookSecret);
    const ok = good && !bad && !tampered && payload.event === "note.created" && payload.note?.id === "key-123" && payload.note?.body === "hello";
    return [ok, { payload, signature: delivery.signature, good, bad, tampered }];
  });
}

// The socket both ways: the client sends `subscribe` (typed), the server answers with events.
const subscribe = socket => new Promise((resolve, reject) => {
  const events = [];
  socket.on("message", message => { events.push(message); if (events.length === 3) resolve(events); });
  socket.on("error", error => reject(new Error(String(error?.message ?? error))));
  socket.on("close", event => reject(new Error(`closed ${event?.code ?? ""} ${event?.reason ?? ""}`)));
});
const expected = (events, authorization) => events.map(event => `${event.event} ${event.id}`).join() === "notes.created n1,notes.created n2,notes.created n3" && events.every(event => event.auth === authorization);

await check("websocket client (typed sendSubscribe, typed events, bearer token sent)", async () => {
  const socket = await client.liveNotes.connect({ reconnectAttempts: 0, connectionTimeoutInSeconds: 10 });
  const done = subscribe(socket);
  await socket.waitForOpen();
  socket.sendSubscribe({ topic: "notes" });
  const events = await done;
  socket.close();
  return [expected(events, bearer()), events];
});

await check("websocket with the token as ?access_token= (what a browser or a Worker can send)", async () => {
  const { access_token } = await client.auth.getToken({ client_id: credentials.clientId, client_secret: credentials.clientSecret });
  // A browser's WebSocket: no headers.
  const socket = new WebSocket(`${wsBase}/notes/live?access_token=${encodeURIComponent(access_token)}`);
  const events = await new Promise((resolve, reject) => {
    const events = [];
    socket.onopen = () => socket.send(JSON.stringify({ topic: "notes" }));
    socket.onmessage = message => { events.push(JSON.parse(message.data)); if (events.length === 3) resolve(events); };
    socket.onerror = () => reject(new Error("the socket failed"));
    socket.onclose = event => reject(new Error(`closed ${event.code} ${event.reason}`));
  });
  socket.close();
  return [expected(events, `Bearer ${access_token}`), events];
});

await check("audiences: the public SDK has notes and auth, not the internal upload", async () => {
  const resource = name => existsSync(new URL(`typescript-public/api/resources/${name}`, out));
  if (!existsSync(new URL("typescript-public", out))) return [false, "no sdk/out/typescript-public: mise run sdk:gen typescript-public"];
  return [resource("notes") && resource("auth") && !resource("files") && existsSync(new URL("typescript-dist/esm/api/resources/files", out)), { notes: resource("notes"), files: resource("files") }];
});

if (!open) {
  // The contract's security is enforced, not only declared.
  const raw = (path, init = {}) => fetch(`${base}${path}`, { ...init, signal: AbortSignal.timeout(TIMEOUT) });
  await check("a call without a token, or with a forged one, is 401", async () => {
    const none = await raw("/notes");
    const forged = await raw("/notes", { headers: { authorization: `${bearer().slice(0, -4)}0000` } });
    const wrong = await raw("/oauth/token", { method: "POST", body: new URLSearchParams({ client_id: "id-1", client_secret: "nope" }) });
    const valid = await raw("/notes", { headers: { authorization: bearer() } });
    return [none.status === 401 && forged.status === 401 && wrong.status === 401 && valid.status === 200, { none: none.status, forged: forged.status, wrongSecret: wrong.status, valid: valid.status }];
  });
  await check("a websocket without a token is refused, and a message the contract rejects closes it (1008)", async () => {
    // The refusal is Go's 401. cf dev's server drops the connection instead of passing it on (an error, no status).
    const refused = await new Promise(resolve => {
      const socket = new RawWebSocket(`${wsBase}/notes/live`);
      socket.on("unexpected-response", (_, response) => { resolve(response.statusCode); socket.terminate(); });
      socket.on("open", () => { resolve("opened"); socket.close(); });
      socket.on("error", error => resolve(String(error.message)));
    });
    const closed = await new Promise(resolve => {
      const socket = new RawWebSocket(`${wsBase}/notes/live`, { headers: { authorization: bearer() } });
      socket.on("open", () => socket.send(JSON.stringify({ subject: "not a subscribe message" })));
      socket.on("message", data => resolve(`a frame: ${data}`));
      socket.on("close", code => resolve(code));
      socket.on("error", error => resolve(String(error.message)));
    });
    return [(refused === 401 || (typeof refused === "string" && refused !== "opened")) && closed === 1008, { refused, closed }];
  });
}

receiver?.close();
console.log(failed ? `${failed} failed against ${base}` : `all passed against ${base} (SDK: sdk/out)`);
process.exit(failed ? 1 : 0);
