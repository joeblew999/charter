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
//   GET  /follow?after=&as=&seconds=&end= the rest of an API stream that Go began: see below
//
// A page's stream is held here, not by a Go runtime. A Go runtime takes some 13 MB or more of the Worker's 128 MB
// for as long as it serves a request, and a page's stream is a request that lasts minutes: a handful of open pages
// took a Worker over its memory, and Cloudflare ended it with every request it was answering. So the stream a page
// holds open tells it one thing: what you show has changed (the event "changed"). The page then asks for that part
// of itself again, which is one short, ordinary request that Go renders for that reader, in their language, with
// what they may do, and puts what comes in place of what it had. It is told that something changed, never what:
// so what was added, taken away or altered all reach it the same way, nothing is shown twice, and however many
// changes come together, the page ends as the log is.
//
// Nothing is lost when a stream breaks. A page says when it was rendered (?at=, in milliseconds), and a stream
// that connects again says when it was last told (Last-Event-ID); the hub answers "changed" at once when
// something changed since then, or when it cannot know because it started since then itself, as it does on a
// deploy.
//
// The API's streams are held here too, for the same reason. Go answers such a request with what the log has after
// the position its caller gave, says where it stopped, and is done; worker/go.mjs then joins this object's /follow
// to that answer, which sends every item published after that position, as Go would have written it: a server-sent
// event for an SDK client (as=events, for `seconds`, then the event `close` with `end` as its data), a line of JSON
// for a WebSocket channel (as=lines, until its reader goes). An item published in the moment between Go's reading
// the log and this stream's start is in what this object remembers of the last ones, and is sent first.
// Upstream: syumai/workers-go#220 (when fixed: the hub can be a Go Durable Object)
import { DurableObject } from "cloudflare:workers";

// How long a page's stream stays open: then it ends, and the browser connects again from the last position it was
// told of. A stream never outlives a deploy by long.
const LIFETIME = 5 * 60 * 1000;
// A comment this often keeps what is between the Worker and the browser from closing a quiet stream.
const BEAT = 25 * 1000;
// What a page's stream says: the name of its one event, which the page's markup listens for (live.Changed in Go).
const CHANGED = "changed";
// How far two clocks may be apart, the Worker's and this hub's: a change this little before a page was rendered is
// taken as after it.
const SLACK = 10 * 1000;
// How many of the last published items are remembered, for a stream that starts a moment after one of them.
const REMEMBERED = 200;
const encoder = new TextEncoder();

export class Hub extends DurableObject {
	// Streams that are open: a page's (its controller), or an API stream's (its controller, the last position it was
	// sent, and how it is written). In memory only: a hub that restarts closes them, and they connect again.
	readers = new Set();
	// When this hub started, and when it last knew of a change: what a page that connects is measured against.
	startedAt = Date.now();
	changedAt = 0;
	// The last items published, each with its position: in memory only.
	recent = [];

	// publish is what the Go hub calls (Workers RPC): a method call costs the caller less CPU than
	// a fetch, which has a Request and a Response to make. position is the item's place in the log,
	// when it has one: what an API stream is sent from. Every open page is told that something changed.
	publish(message, position) {
		if (Number.isFinite(position)) {
			this.recent.push({ position, message });
			if (this.recent.length > REMEMBERED) this.recent.shift();
			for (const reader of this.readers) if (reader.kind !== "page" && position > reader.last) this.tell(reader, position, message);
		}
		this.changed();
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

	// changed tells every open page that what it shows has changed: what the Go hub calls for a change that is no
	// new item (one taken away, one altered), and what publish does for a new one.
	changed() {
		this.changedAt = Date.now();
		let told = 0;
		for (const reader of this.readers) {
			if (reader.kind !== "page") continue;
			this.say(reader);
			told++;
		}
		return told;
	}

	// say tells one page that what it shows has changed. The id is when: what the page says if it connects again.
	say(reader) {
		this.write(reader, `id: ${Date.now()}\nevent: ${CHANGED}\ndata:\n\n`);
	}

	// tell gives an API stream the item published at a position, as an event or as a line.
	tell(reader, position, message) {
		reader.last = position;
		if (reader.kind === "events") return this.write(reader, `event: message\nretry: 1000\nid: ${position}\ndata: ${message}\n\n`);
		this.write(reader, `${message}\n`);
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

	// follow answers the rest of an API stream: everything published after a position, as events or as lines.
	follow(request, url) {
		const kind = url.searchParams.get("as");
		if (kind !== "events" && kind !== "lines") return new Response("a stream is followed as=events or as=lines", { status: 400 });
		const last = Number(url.searchParams.get("after")) || 0;
		const seconds = Number(url.searchParams.get("seconds")) || 0;
		const end = url.searchParams.get("end");
		let reader;
		const body = new ReadableStream({
			start: controller => {
				reader = { controller, last, kind };
				// What was published since Go read the log, then what is published from now on.
				for (const item of this.recent) if (item.position > reader.last) this.tell(reader, item.position, item.message);
				this.readers.add(reader);
				if (kind === "events") {
					this.beat ??= setInterval(() => {
						for (const each of this.readers) if (each.kind !== "lines") this.write(each, ": \n\n");
					}, BEAT);
				}
				// A planned end: the terminator a generated client stops at, then the stream's own end.
				if (seconds > 0) {
					reader.end = setTimeout(() => {
						if (kind === "events" && end !== null) this.write(reader, `event: close\ndata: ${end}\n\n`);
						this.gone(reader);
					}, seconds * 1000);
				}
			},
			cancel: () => this.gone(reader),
		});
		request.signal?.addEventListener("abort", () => reader && this.gone(reader), { once: true });
		return new Response(body);
	}

	// signal answers a page's stream.
	signal(request, url) {
		// When the page last knew what it shows was current: when this stream last told it, if it connects again,
		// else when it was rendered.
		const again = request.headers.get("Last-Event-ID");
		const at = Number(again ?? url.searchParams.get("at")) || 0;
		let reader;
		const body = new ReadableStream({
			start: controller => {
				reader = { controller, kind: "page" };
				this.readers.add(reader);
				reader.end = setTimeout(() => this.gone(reader), LIFETIME);
				this.beat ??= setInterval(() => {
					for (const each of this.readers) if (each.kind !== "lines") this.write(each, ": \n\n");
				}, BEAT);
				// Something changed since then, or this hub started since then and cannot know: the page is told at
				// once. A clock is not another's to the millisecond (the page's time is the Worker's, this one the
				// hub's), so a change a little before counts too: a page told once too often asks once too often.
				if (Math.max(this.changedAt, this.startedAt) > at - SLACK) this.say(reader);
				// Else when it was, as an id and nothing else: the response starts now, and a stream that breaks before
				// anything changes connects again saying where the page was. A browser keeps an id that comes alone and
				// shows nothing for it.
				else this.write(reader, `id: ${at}\n\n`);
			},
			cancel: () => this.gone(reader),
		});
		request.signal?.addEventListener("abort", () => reader && this.gone(reader), { once: true });
		return new Response(body, { headers: { "Content-Type": "text/event-stream", "Cache-Control": "no-store" } });
	}

	async fetch(request) {
		const url = new URL(request.url);
		if (request.method === "GET" && url.pathname === "/signal") return this.signal(request, url);
		if (request.method === "GET" && url.pathname === "/follow") return this.follow(request, url);
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
