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

// The page: Datastar, and the list following the stream from its newest message.
const home = await fetch(`${origin}/`);
const page = await home.text();
const follows = page.match(/data-init="@get\(&#39;(\/messages\/stream\?after=\d+)&#39;/)?.[1];
check("GET / is the page, following the stream", home.status === 200 && page.includes('<script type="module" src="/static/datastar-1.0.4.js">') && follows, { status: home.status, page: page.slice(0, 300) });
const js = await fetch(`${origin}/static/datastar-1.0.4.js`);
check("GET /static/datastar-1.0.4.js", js.status === 200 && js.headers.get("content-type")?.startsWith("text/javascript") && js.headers.get("cache-control")?.includes("immutable") && (await js.text()).length > 1000, { status: js.status });

// The stream, as the page opens it: a message posted from the form, then one through the API,
// each arrives as a datastar-patch-elements event with its HTML fragment, whose id is the message's.
const stream = new AbortController();
const response = await fetch(`${origin}${follows ?? "/messages/stream"}`, { signal: stream.signal, headers: { accept: "text/event-stream, text/html, application/json", "datastar-request": "true" } });
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
// The form as Datastar posts it (@post, contentType form).
const post = body => fetch(`${origin}/messages`, { method: "POST", headers: { "datastar-request": "true" }, body: new URLSearchParams({ body }) });
const form = await post(`from the form ${stamp} <b>`);
const answer = await form.text();
check("POST /messages answers the empty form, to replace the one sent", form.status === 200 && form.headers.get("datastar-mode") === "replace" && answer.startsWith('<form id="post"') && answer.includes('value=""'), { status: form.status, answer });
const patch = "event: datastar-patch-elements\ndata: selector #messages\ndata: mode prepend\ndata: elements ";
let event = await nextEvent();
check("the stream sends it as its fragment, escaped", /^id: \d+\n/.test(event ?? "") && event.includes(`${patch}<li id="message-`) && event.includes(`<span>from the form ${stamp} &lt;b&gt;</span>`), event);

const created = await fetch(`${origin}/api/messages`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ body: `from the API ${stamp}` }) });
const message = await created.json().catch(() => null);
check("POST /api/messages", created.status === 200 && message?.body === `from the API ${stamp}`, { status: created.status, message });
event = await nextEvent();
check("the stream sends that one too, with its id", event?.startsWith(`id: ${message?.id}\n${patch}<li id="message-${message?.id}">`), event);

// Refused: the form and why, as a 200, since Datastar patches nothing from another status.
const refused = await post("   ");
check("POST /messages refuses an empty message with the form and why", refused.status === 200 && (await refused.text()).includes('class="problem"'), { status: refused.status });

stream.abort();
process.exit(failed ? 1 : 0);
