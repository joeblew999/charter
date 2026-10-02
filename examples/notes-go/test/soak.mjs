// The real-time test matrix (docs/realtime.md): every client × scenario against the deployed Worker.
// Each client follows the one client rule: when a stream ends (planned, error or drop), call again
// with `after` = the last note id. The server must make every scenario gap-free:
//   steady state  - a note every 2 s
//   planned end   - streams end every --stream-seconds (SSE), and clients come back with `after`
//   hub restart   - the Worker is redeployed at --deploy-at (restarts the hub Durable Object)
//   client drop   - at --drop-at every client disconnects for 6 s, then resumes with `after`
//   long idle     - `--idle <minutes>`: no notes for that long, then one note
// PASS = every note created, exactly once, in id order. Latency (creation to receipt) is reported,
// not judged: the generated CLI prints json/jsonl only when a stream ends.
// Upstream: fern-api/fern#17939 (when fixed: the CLI's latency drops to ~0 like the SDKs'; judge it)
//
//   node soak.mjs <origin> [--no-deploy] [--seconds 100] [--deploy-at 30] [--drop-at 65] [--stream-seconds 15]
//   node soak.mjs <origin> --idle 20
// Run it from the project's folder. It uses that project's `ws` (node_modules) and generated SDKs:
// the TypeScript SDK (sdk/out/typescript-dist), the CLI (sdk/out/cli) and the Go SDK (sdk/out/go)
// through soak-go beside this file; the redeploy runs `mise run deploy` there.
import { execFileSync, spawn } from "node:child_process";
import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { createInterface } from "node:readline";
import { pathToFileURL } from "node:url";
const root = `${process.cwd()}/`; // the project
const WebSocket = createRequire(root)("ws");

const args = process.argv.slice(2);
const origin = args[0];
const str = (name, def) => { const i = args.indexOf(`--${name}`); return i < 0 ? def : args[i + 1]; };
const opt = (name, def) => Number(str(name, def));
const idleMinutes = opt("idle", 0);
const total = (idleMinutes ? idleMinutes * 60 + 15 : opt("seconds", 100)) * 1000;
const deployAt = idleMinutes || args.includes("--no-deploy") ? Infinity : opt("deploy-at", 30) * 1000;
const dropAt = idleMinutes ? Infinity : opt("drop-at", 65) * 1000;
const streamSeconds = opt("stream-seconds", idleMinutes ? 60 : 15);
const { NotesClient } = await import(pathToFileURL(`${root}sdk/out/typescript-dist/esm/index.mjs`));
const t0 = Date.now();
const t = () => ((Date.now() - t0) / 1000).toFixed(1);
const sleep = ms => new Promise(r => setTimeout(r, ms));
const tag = `soak ${t0}`;
let stopping = false;

// The position every client starts from: the newest note now.
const baseline = (await (await fetch(`${origin}/api/notes?limit=1`)).json()).data[0]?.id ?? 0;

// ---- clients: run(after, signal, onNote) resolves when its stream/socket ends ----

async function* sseBlocks(res) {
  const decoder = new TextDecoder(); let buf = "";
  for await (const chunk of res.body) {
    buf += decoder.decode(chunk, { stream: true });
    const blocks = buf.split("\n\n"); buf = blocks.pop();
    for (const block of blocks) {
      const field = f => new RegExp(`^${f}: ?(.*)$`, "m").exec(block)?.[1];
      yield { id: field("id"), event: field("event"), data: field("data") };
    }
  }
}
async function sseRun(url, headers, signal, onNote) {
  const res = await fetch(url, { headers: { accept: "text/event-stream", ...headers }, signal });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  for await (const e of sseBlocks(res)) {
    if (e.event === "message" && e.data) onNote(JSON.parse(e.data));
    else if (e.event === "error") return `event: error ${e.data ?? ""}`;
  }
  return "end";
}
function lines(proc, signal, onLine) {
  signal.addEventListener("abort", () => proc.kill());
  createInterface({ input: proc.stdout }).on("line", onLine);
  let err = ""; proc.stderr.on("data", d => err += d);
  return new Promise(resolve => proc.on("exit", code => resolve(`exit ${code}${err.trim() ? ` ${err.trim().slice(0, 100)}` : ""}`)));
}
const ws = origin.replace(/^http/, "ws");
const soakGo = new URL("soak-go/", import.meta.url).pathname;
const goBin = `${soakGo}soak-go`;
const cliBin = `${root}sdk/out/cli/target/release/notes`;

const clients = [
  { name: "SSE raw (after)", run: (after, signal, onNote) =>
      sseRun(`${origin}/api/notes/watch?after=${after}&seconds=${streamSeconds}`, {}, signal, onNote) },
  { name: "SSE EventSource-style (Last-Event-ID)", run: (after, signal, onNote) =>
      sseRun(`${origin}/api/notes/watch?seconds=${streamSeconds}`, { "last-event-id": String(after) }, signal, onNote) },
  { name: "TypeScript SDK notes.watch()", run: async (after, signal, onNote) => {
      const stream = await new NotesClient({ baseUrl: origin }).notes.watch({ after: String(after), seconds: streamSeconds }, { abortSignal: signal });
      for await (const note of stream) onNote(note);
      return "end";
    } },
  { name: "Go SDK Notes.Watch()", run: (after, signal, onNote) =>
      lines(spawn(goBin, ["-base", origin, "-after", String(after), "-seconds", String(streamSeconds)]), signal, l => onNote(JSON.parse(l))) },
  { name: "CLI notes watch", run: (after, signal, onNote) =>
      lines(spawn(cliBin, ["--base-url", origin, "notes", "watch", "--after", String(after), "--seconds", String(streamSeconds), "--format", "jsonl"]), signal, l => {
        if (l.startsWith("{")) onNote(JSON.parse(l));
      }) },
  { name: "WebSocket raw (?after)", run: (after, signal, onNote) => new Promise(resolve => {
      const sock = new WebSocket(`${ws}/api/notes/live?after=${after}`);
      signal.addEventListener("abort", () => sock.terminate());
      sock.on("message", data => onNote(JSON.parse(String(data))));
      sock.on("error", () => {});
      sock.on("close", code => resolve(`closed ${code}`));
    }) },
  { name: "TypeScript SDK liveNotes.connect({ after })", run: async (after, signal, onNote) => {
      const sock = await new NotesClient({ baseUrl: ws }).liveNotes.connect({ after: String(after), reconnectAttempts: 0 });
      signal.addEventListener("abort", () => sock.close());
      sock.on("message", onNote);
      return new Promise(resolve => sock.on("close", e => resolve(`closed ${e?.code ?? ""}`)));
    } },
];

// ---- the client rule, the same driver for every client ----

for (const c of clients) Object.assign(c, { last: baseline, seen: new Map(), at: new Map(), order: true, runs: 0, ends: {}, pausedUntil: 0, abort: () => {} });
function record(c, note) {
  if (!note?.body?.startsWith(tag)) return;
  if (note.id <= c.last) c.order = c.order && c.seen.has(note.id); // an older id that was never seen = out of order
  if (!c.seen.has(note.id)) c.at.set(note.id, Date.now());
  c.seen.set(note.id, (c.seen.get(note.id) ?? 0) + 1);
  if (note.id > c.last) c.last = note.id;
}
async function drive(c) {
  while (!stopping) {
    if (Date.now() < c.pausedUntil) { await sleep(200); continue; }
    const ac = new AbortController();
    c.abort = () => ac.abort();
    let why;
    // Upstream: fern-api/fern#17937 (when fixed: resumable SDKs survive resets themselves; the catch stays for the rule)
    try { why = await c.run(c.last, ac.signal, note => record(c, note)); } catch (error) { why = ac.signal.aborted ? "dropped" : `error: ${error.message}`; }
    c.runs++;
    const key = String(why).replace(/\d{3,}/g, "N").slice(0, 60);
    c.ends[key] = (c.ends[key] ?? 0) + 1;
    await sleep(300);
  }
}

function redeploy() {
  console.log(`${t()}s hub restart: redeploying...`);
  return new Promise(resolve => {
    const proc = spawn("mise", ["run", "deploy"], { cwd: root });
    let out = ""; proc.stdout.on("data", d => out += d); proc.stderr.on("data", d => out += d);
    proc.on("exit", code => { console.log(`${t()}s redeploy exited ${code}, version ${/Current Version ID: (\S+)/.exec(out)?.[1] ?? "?"}`); resolve(); });
  });
}

// ---- run ----

// The Go client against the generated SDK: a workspace file that points the SDK's module path (the
// one its go.mod declares, which soak-go imports) at it.
const work = `${mkdtempSync(`${tmpdir()}/soak-go-`)}/go.work`;
const goModule = /^module\s+(\S+)/m.exec(readFileSync(`${root}sdk/out/go/go.mod`, "utf8"))[1];
writeFileSync(work, `go 1.27.1\n\nuse ${soakGo.replace(/\/$/, "")}\n\nreplace ${goModule} => ${root}sdk/out/go\n`);
execFileSync("go", ["build", "-o", goBin, "."], { cwd: soakGo, env: { ...process.env, GOWORK: work } });
console.log(`baseline note ${baseline}; ${clients.length} clients; SSE streams end every ${streamSeconds}s`);
const driving = clients.map(drive);
await sleep(3000);

const created = [];
const create = async n => {
  try {
    const res = await fetch(`${origin}/api/notes`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ body: `${tag} #${n}` }) });
    if (res.ok) created.push({ id: (await res.json()).id, at: t(), ms: Date.now() }); else console.log(`${t()}s create failed ${res.status}`);
  } catch (error) { console.log(`${t()}s create error ${error.message}`); }
};
let deploying, dropped = false;
if (idleMinutes) {
  console.log(`${t()}s idle for ${idleMinutes} min, then one note`);
  await sleep(idleMinutes * 60_000);
  await create(0);
} else {
  console.log(`${t()}s a note every 2s for ${total / 1000}s; redeploy at ${deployAt / 1000}s; client drop at ${dropAt / 1000}s`);
  for (let n = 0; Date.now() - t0 < total; n++) {
    if (!deploying && Date.now() - t0 >= deployAt) deploying = redeploy();
    if (!dropped && Date.now() - t0 >= dropAt) {
      dropped = true; console.log(`${t()}s client drop: every client offline for 6s`);
      for (const c of clients) { c.pausedUntil = Date.now() + 6000; c.abort(); }
    }
    await create(n);
    await sleep(2000);
  }
}
await deploying;
// Let the last notes arrive: the CLI prints json/jsonl only when a stream ends, so wait out one stream.
await sleep((streamSeconds + 5) * 1000);
stopping = true;
for (const c of clients) c.abort();
await Promise.race([Promise.allSettled(driving), sleep(5000)]);

// ---- report ----
console.log(`\n${created.length} notes created (ids ${created[0]?.id}..${created.at(-1)?.id})\n`);
let failed = 0;
for (const c of clients) {
  const missing = created.filter(n => !c.seen.has(n.id));
  const dupes = [...c.seen.values()].filter(n => n > 1).length;
  const ok = missing.length === 0 && dupes === 0 && c.order;
  const lat = created.filter(n => c.at.has(n.id)).map(n => c.at.get(n.id) - n.ms).sort((a, b) => a - b);
  const p = q => lat.length ? `${(Math.max(0, lat[Math.min(lat.length - 1, Math.floor(q * lat.length))]) / 1000).toFixed(1)}s` : "-";
  if (!ok) failed++;
  console.log(`${ok ? "PASS" : "FAIL"}  ${c.name}: ${c.seen.size}/${created.length}, missing ${missing.length}${missing.length ? ` (first at ${missing[0].at}s)` : ""}, duplicates ${dupes}, ${c.order ? "in order" : "OUT OF ORDER"}, latency p50 ${p(0.5)} max ${p(1)}, ${c.runs} connections`);
  console.log(`        ends: ${Object.entries(c.ends).map(([k, v]) => `${k} ×${v}`).join(", ")}`);
}
process.exit(failed ? 1 : 0);
