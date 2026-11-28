// The Worker in front: what is at an app's address. It is a few KB, so Cloudflare has it running in a few
// milliseconds wherever a request arrives, where the Go Worker behind it takes as long to load as it is large
// (measured: 0.2 to 0.6 s for 18 MiB, before a line of it runs). charter deploy writes its configuration
// (internal/command/split.go) and charter build its entry:
//
//	import { front } from "./front.mjs";
//	export default front("<the build>");
//
// It does one thing of its own: a page that is the same for every reader is answered from KV (CHARTER_PAGES),
// which is one copy for every data centre. Everything else is the Go Worker's (CHARTER_GO, a service binding), asked
// as it was asked: a reader with a cookie or a token, the API, a part of a page, a stream, a WebSocket.
//
// Which page may be kept is not decided here. Go says so on its answer (Cache-Control: public, app/shared.go), and
// this keeps what Go says may be kept, for the requests Go would have said it of. This is Void's ISR (its
// packages/isr) with Cloudflare's cache for the Worker as the layer before it: that cache is asked before this
// runs and has a page in the data centres that were asked for it; KV has it for all the others.
//
// A copy is kept under the build that drew it, so a deploy starts with none: nothing is deleted, the new build
// looks under its own name, and what an older one kept goes after a month.

// How long a build's pages stay in KV: long after another build has replaced it.
const MONTH = 30 * 24 * 60 * 60;
// What of Go's answer is not kept with the page: what this says itself, and what is of that one answer.
const UNSAID = ["content-length", "content-encoding", "transfer-encoding", "connection", "date", "age", "x-charter-build", "x-go-runtime"];
// Which build an answer is of, on every answer (charter build's entry for the Go Worker says the same).
const BUILD = "x-charter-build";

export function front(build) {
	return {
		async fetch(request, env, ctx) {
			const key = keyOf(request, build);
			if (key) {
				const kept = await env.CHARTER_PAGES.getWithMetadata(key, "stream").catch(() => null);
				if (kept?.value && kept.metadata) {
					const headers = new Headers(kept.metadata);
					headers.set(BUILD, build);
					headers.set("x-charter-kept", "kv");
					return new Response(kept.value, { headers });
				}
			}
			const response = await env.CHARTER_GO.fetch(request);
			if (key && keepable(response, build)) {
				ctx.waitUntil(keep(env.CHARTER_PAGES, key, response.clone()));
			}
			return response;
		},
	};
}

// keyOf is what a request's page is kept under, undefined for a request whose answer is its asker's or is no
// whole page: one with a cookie, a token or a database bookmark, one htmx made for a part of a page, anything but
// a GET over https of an address with no query. The rules are Go's (app/shared.go's anyone and share), said again
// here so that such a request is never given a kept page; what Go would not keep, it does not say may be kept.
function keyOf(request, build) {
	if (request.method !== "GET") return;
	const { protocol, host, pathname, search } = new URL(request.url);
	if (protocol !== "https:" || search) return;
	const has = name => request.headers.has(name);
	if (has("cookie") || has("authorization") || has("x-d1-newest") || has("upgrade")) return;
	// htmx names what it will replace: the whole page (a boosted link, "body") is the page; anything else is a part.
	const target = (request.headers.get("hx-target") ?? "").split("#")[0];
	if (target && target !== "body") return;
	// A page as it is loaded and as a link in the app gets it are two answers: for a page that must be a page
	// load, the second is empty and tells htmx to load it (HX-Redirect, app/guard.go).
	const key = `${build}:${target ? "link" : "page"}:${host}${pathname}`;
	if (key.length > 512) return; // the longest name KV takes
	return key;
}

// keepable is whether Go's answer is a page every reader may be given, drawn by this build: while a deploy is
// reaching Cloudflare's data centres the Go Worker asked may still be the one before, and its page is not kept
// under this build's name.
function keepable(response, build) {
	return (
		response.status === 200 &&
		(response.headers.get("cache-control") ?? "").startsWith("public") &&
		!response.headers.has("set-cookie") &&
		response.headers.get(BUILD) === build
	);
}

// keep puts a copy of an answer in KV, with what it said of itself. A failure keeps nothing, and the next reader's
// answer is tried again.
async function keep(pages, key, copy) {
	// KV takes 1024 bytes of it: an answer that says more than that of itself is not kept (the put fails).
	const metadata = {};
	for (const [name, value] of copy.headers) {
		if (!UNSAID.includes(name) && !name.startsWith("cf-")) metadata[name] = value;
	}
	try {
		await pages.put(key, await copy.arrayBuffer(), { metadata, expirationTtl: MONTH });
	} catch {
		// Nothing is kept.
	}
}
