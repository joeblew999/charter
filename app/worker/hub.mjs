// Hub: a live fan-out as a Durable Object, what the Go package hub publishes to and subscribes
// from (the counterpart of the TypeScript projects' Hub, in their worker.ts). Subscribers (a Go Worker's Follow loops) are
// hibernatable WebSockets, so the hub sleeps between messages. It stores nothing and relays any
// message as it is: the log is the Worker's own (D1 in the notes example) and Follow catches up
// from it, so the hub may restart at any time (docs/self/streaming.md, rule 2). One class serves every
// feed: each feed is an object of it, by name. It is JavaScript because a Durable Object class has
// to be: workers-go can call one, not be one.
//
// A project's entry exports it under the name its charter.toml declares ([[durable_objects]] class):
//
//	export { Hub } from "./build/hub.mjs";
//
//   GET  /subscribe  (WebSocket upgrade)  every published message arrives as one text frame
//   publish(message, position)            the message is sent to every subscriber as it is
//   POST /publish                         the same, with the message as the body
//   GET  /signal?after=&get=&as=          a page's stream (server-sent events): see below
//
// A page's stream is held here, not by a Go runtime. A Go runtime takes some 13 MB or more of the Worker's 128 MB
// for as long as it serves a request, and a page's stream is a request that lasts minutes: a handful of open pages
// took a Worker over its memory, and Cloudflare ended it with every request it was answering. So the stream a page
// holds open tells it only that there is something new, and where to ask for it: each event is one element that
// htmx puts in the page, and which then asks for itself (hx-get, on load) what came after the position the page
// has. That is one short, ordinary request, which Go renders for that reader, in their language, with what they
// may do. The log stays the truth, and nothing is lost: a page that connects again says the last position it was
// told of (Last-Event-ID), and is told to ask from there.
// Upstream: syumai/workers-go#220 (when fixed: the hub can be a Go Durable Object)
import { DurableObject } from "cloudflare:workers";

// How long a page's stream stays open: then it ends, and the browser connects again from the last position it was
// told of. A stream never outlives a deploy by long.
const LIFETIME = 5 * 60 * 1000;
// A comment this often keeps what is between the Worker and the browser from closing a quiet stream.
const BEAT = 25 * 1000;
// What a page may ask to be told to fetch: a path of the app, and the kind of element to say it with.
const PATH = /^\/[A-Za-z0-9._~\/-]*$/;
const ELEMENTS = new Set(["div", "li", "tr", "span", "p", "section"]);
const encoder = new TextEncoder();

export class Hub extends DurableObject {
	// Pages with a stream open: each its stream's controller, the last position it was told of, and what it asked
	// to be told to fetch. In memory only: a hub that restarts closes their streams, and they connect again.
	readers = new Set();

	// publish is what the Go hub calls (Workers RPC): a method call costs the caller less CPU than
	// a fetch, which has a Request and a Response to make. position is the item's place in the log,
	// when it has one: what a page's stream is told.
	publish(message, position) {
		if (Number.isFinite(position)) for (const reader of this.readers) if (position > reader.last) this.tell(reader, position);
		let sent = 0;
		for (const socket of this.ctx.getWebSockets()) {
			try {
				socket.send(message);
				sent++;
			} catch {
				// Closing: the runtime drops it from getWebSockets().
			}
		}
		return sent;
	}

	// tell says to a page that there is something after the position it has: an element that asks for it.
	tell(reader, position) {
		const element = `<${reader.as} hidden hx-get="${reader.get}?after=${reader.last}" hx-trigger="load" hx-swap="outerHTML"></${reader.as}>`;
		reader.last = position;
		this.write(reader, `id: ${position}\ndata: ${element}\n\n`);
	}

	write(reader, text) {
		try {
			reader.controller.enqueue(encoder.encode(text));
		} catch {
			this.gone(reader);
		}
	}

	gone(reader) {
		this.readers.delete(reader);
		clearTimeout(reader.end);
		try {
			reader.controller.close();
		} catch {
			// Closed already.
		}
		if (this.readers.size === 0 && this.beat) {
			clearInterval(this.beat);
			this.beat = undefined;
		}
	}

	// signal answers a page's stream.
	signal(request, url) {
		const get = url.searchParams.get("get") ?? "";
		const as = url.searchParams.get("as") ?? "div";
		if (!PATH.test(get) || !ELEMENTS.has(as)) return new Response("a stream is asked with get=<a path of the app> and as=<an element>", { status: 400 });
		// Where the page is: the last position this stream told it of, when it connects again, else the one it was
		// rendered with.
		const again = request.headers.get("Last-Event-ID");
		const last = Number(again ?? url.searchParams.get("after")) || 0;
		let reader;
		const body = new ReadableStream({
			start: controller => {
				reader = { controller, last, get, as };
				this.readers.add(reader);
				reader.end = setTimeout(() => this.gone(reader), LIFETIME);
				this.beat ??= setInterval(() => {
					for (const each of this.readers) this.write(each, ": \n\n");
				}, BEAT);
				// A comment first: the response starts now, not at the first item.
				this.write(reader, ": \n\n");
				// Back after a break: whatever came meanwhile is asked for at once.
				if (again !== null) this.tell(reader, last);
			},
			cancel: () => this.gone(reader),
		});
		request.signal?.addEventListener("abort", () => reader && this.gone(reader), { once: true });
		return new Response(body, { headers: { "Content-Type": "text/event-stream", "Cache-Control": "no-cache" } });
	}

	async fetch(request) {
		const url = new URL(request.url);
		if (request.method === "GET" && url.pathname === "/signal") return this.signal(request, url);
		if (request.method === "POST") return Response.json({ sent: this.publish(await request.text()) });
		if (request.headers.get("upgrade") !== "websocket") return new Response("expected a WebSocket upgrade", { status: 426 });
		const [client, server] = Object.values(new WebSocketPair());
		this.ctx.acceptWebSocket(server);
		return new Response(null, { status: 101, webSocket: client });
	}

	// Subscribers only listen.
	webSocketMessage() {}

	// Complete the close handshake (1005/1006 can't be sent back in a close frame).
	webSocketClose(socket, code, reason) {
		try {
			socket.close(code === 1005 || code === 1006 ? 1000 : code, reason);
		} catch {
			// Already closed.
		}
	}
}
