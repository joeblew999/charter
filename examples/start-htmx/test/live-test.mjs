// Live test of the API and its pages: what a deploy must pass, run against a local server or the
// deployed Worker. It posts two messages. Add a check for each route or page you add.
// Usage, from the project's folder: node test/live-test.mjs <origin>
const origin = process.argv[2];
let failed = 0;
function check(name, ok, detail) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok ? "" : `: ${JSON.stringify(detail)}`}`);
  if (!ok) failed++;
}

const hello = await fetch(`${origin}/api/hello`);
const body = await hello.json().catch(() => null);
check("GET /api/hello", hello.status === 200 && body?.message?.startsWith("Hello from "), { status: hello.status, body });

const spec = await fetch(`${origin}/api/openapi.json`);
const openapi = await spec.json().catch(() => null);
check("GET /api/openapi.json names this origin", spec.status === 200 && openapi?.servers?.[0]?.url === origin, { status: spec.status, servers: openapi?.servers });

// The page: htmx 4 and its SSE extension, and the list following the stream from its newest message.
const home = await fetch(`${origin}/`);
const page = await home.text();
const follows = page.match(/hx-sse:connect="(\/messages\/stream\?after=\d+)"/)?.[1];
check("GET / is the page, following the stream", home.status === 200 && page.includes('<script src="/static/htmx-4.0.0.min.js">') && follows, { status: home.status, page: page.slice(0, 300) });
for (const file of ["/static/htmx-4.0.0.min.js", "/static/hx-sse-4.0.0.min.js"]) {
  const js = await fetch(`${origin}${file}`);
  check(`GET ${file}`, js.status === 200 && js.headers.get("content-type")?.startsWith("text/javascript") && (await js.text()).length > 1000, { status: js.status });
}

// The stream, as the page opens it: a message posted from the form, then one through the API,
// each arrives as its HTML fragment, in an event whose id is the message's.
const stream = new AbortController();
const response = await fetch(`${origin}${follows ?? "/messages/stream"}`, { signal: stream.signal, headers: { accept: "text/html, text/event-stream" } });
check("GET /messages/stream is SSE", response.status === 200 && response.headers.get("content-type") === "text/event-stream", { status: response.status, type: response.headers.get("content-type") });
const reader = response.body.getReader();
const decoder = new TextDecoder();
let buffer = "";
async function nextEvent() {
  const deadline = Date.now() + 15000;
  while (Date.now() < deadline) {
    const end = buffer.indexOf("\n\n");
    if (end >= 0) {
      const event = buffer.slice(0, end);
      buffer = buffer.slice(end + 2);
      if (!event.startsWith(":")) return event;
      continue;
    }
    const timeout = new Promise(resolve => setTimeout(() => resolve({ timedOut: true }), deadline - Date.now()));
    const read = await Promise.race([reader.read(), timeout]);
    if (read.timedOut || read.done) return null;
    buffer += decoder.decode(read.value, { stream: true });
  }
  return null;
}

const stamp = `${Date.now()}`;
const form = await fetch(`${origin}/messages`, { method: "POST", body: new URLSearchParams({ body: `from the form ${stamp} <b>` }) });
const answer = await form.text();
check("POST /messages answers the empty form", form.status === 200 && answer.startsWith('<form id="post"') && answer.includes('value=""'), { status: form.status, answer });
let event = await nextEvent();
check("the stream sends it as its fragment, escaped", /^id: \d+\ndata: <li id="message-\d+"><span>from the form \d+ &lt;b&gt;<\/span>/.test(event ?? "") && event.includes(stamp), event);

const created = await fetch(`${origin}/api/messages`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ body: `from the API ${stamp}` }) });
const message = await created.json().catch(() => null);
check("POST /api/messages", created.status === 200 && message?.body === `from the API ${stamp}`, { status: created.status, message });
event = await nextEvent();
check("the stream sends that one too, with its id", event?.startsWith(`id: ${message?.id}\ndata: <li id="message-${message?.id}">`), event);

const refused = await fetch(`${origin}/messages`, { method: "POST", body: new URLSearchParams({ body: "   " }) });
check("POST /messages refuses an empty message with the form and why (422)", refused.status === 422 && (await refused.text()).includes('class="problem"'), { status: refused.status });

stream.abort();
process.exit(failed ? 1 : 0);
