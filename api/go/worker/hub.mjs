// NotesHub: the live fan-out for new notes, the Go Worker's counterpart of api/ts/src/hub.ts. Subscribers
// (the Go Worker's Follow loops) are hibernatable WebSockets, so the hub sleeps between notes. It
// stores nothing: D1 is the log and Follow catches up from it, so the hub may restart at any time
// (docs/realtime.md, rule 2). It is JavaScript because a Durable Object class has to be: workers-go
// can call one, not be one.
//
//   GET  /subscribe  (WebSocket upgrade)  every published message arrives as one text frame
//   POST /publish                         the body is sent to every subscriber as it is
// Upstream: syumai/workers-go#220 (when fixed: the hub can be a Go Durable Object)
import { DurableObject } from "cloudflare:workers";

export class NotesHub extends DurableObject {
	async fetch(request) {
		if (request.method === "POST") {
			const message = await request.text();
			let sent = 0;
			for (const socket of this.ctx.getWebSockets()) {
				try {
					socket.send(message);
					sent++;
				} catch {
					// Closing: the runtime drops it from getWebSockets().
				}
			}
			return Response.json({ sent });
		}
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
