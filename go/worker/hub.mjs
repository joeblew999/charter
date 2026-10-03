// Hub: a live fan-out as a Durable Object, what the Go package hub publishes to and subscribes
// from (the counterpart of api/ts/src/hub.ts). Subscribers (a Go Worker's Follow loops) are
// hibernatable WebSockets, so the hub sleeps between messages. It stores nothing and relays any
// message as it is: the log is the Worker's own (D1 in the notes example) and Follow catches up
// from it, so the hub may restart at any time (docs/guides/streaming.md, rule 2). One class serves every
// feed: each feed is an object of it, by name. It is JavaScript because a Durable Object class has
// to be: workers-go can call one, not be one.
//
// A project's entry exports it under the name its cloudflare.config.ts declares:
//
//	export { Hub } from "./build/hub.mjs";
//
//   GET  /subscribe  (WebSocket upgrade)  every published message arrives as one text frame
//   publish(message)                      the message is sent to every subscriber as it is
//   POST /publish                         the same, with the message as the body
// Upstream: syumai/workers-go#220 (when fixed: the hub can be a Go Durable Object)
import { DurableObject } from "cloudflare:workers";

export class Hub extends DurableObject {
	// publish is what the Go hub calls (Workers RPC): a method call costs the caller less CPU than
	// a fetch, which has a Request and a Response to make.
	publish(message) {
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

	async fetch(request) {
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
