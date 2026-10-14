// Upstream: syumai/workers-go#97 (when fixed: answer the upgrade in Go and delete this adapter)
// The WebSocket adapter, for any Worker whose Go answers upgrades the way ../transport describes
// (Serve in ../transport/transport_other.go is the same adapter for the native build). Go answers a WebSocket
// upgrade with plain HTTP, and this carries it over the socket:
//
//   - anything but 2xx is a refusal (401, 422, 426), returned as it is;
//   - 200 with a body is the feed: a stream of lines, each sent as one text frame (empty lines are
//     padding). Go's feed only ends when it gave up (the hub is down), so the socket is then closed
//     with 1011 and the client reconnects: it never stays open with nothing behind it;
//   - 204 means there is no feed: the socket stays open until the client closes it;
//   - the header `X-Websocket-Messages: post` means the channel takes messages: each text frame from
//     the client is given to Go as a POST to the same URL, with the upgrade's headers and the frame
//     as the JSON body, and the lines of the answer are sent as frames. One message at a time, in
//     order. Anything but 2xx closes the socket with 1008. Without the header, what the client
//     sends is ignored.
//
// Which paths are WebSockets, their input, their feed and their messages are all Go's business.
// Every call to Go may be in a Go runtime of its own (go.mjs), so a message can't count on reaching
// the feed through memory: what they share goes through a binding.
export async function webSocket(goWorker, request, env, ctx) {
	const answer = await goWorker.fetch(request, env, ctx);
	if (!answer.ok) return answer;
	const feed = answer.status === 204 ? null : answer.body;
	if (!feed && answer.status !== 204) return answer;
	const [client, server] = Object.values(new WebSocketPair());
	server.accept();
	const close = (code, reason) => {
		try {
			server.close(code, reason);
		} catch {
			// The client closed first.
		}
	};

	if (answer.headers.get("x-websocket-messages") === "post") {
		const url = request.url;
		const headers = new Headers(request.headers);
		for (const name of [...headers.keys()]) if (name === "upgrade" || name === "connection" || name.startsWith("sec-websocket-")) headers.delete(name);
		headers.set("content-type", "application/json");
		let queue = Promise.resolve();
		server.addEventListener("message", event => {
			queue = queue
				.then(async () => {
					if (typeof event.data !== "string") return close(1003, "text frames only");
					const reply = await goWorker.fetch(new Request(url, { method: "POST", headers, body: event.data }), env, ctx);
					if (!reply.ok) {
						await reply.body?.cancel();
						return close(1008, `message refused: HTTP ${reply.status}`);
					}
					if (reply.body) await frames(reply.body.pipeThrough(new TextDecoderStream()).getReader(), server);
				})
				.catch(() => close(1011, "message failed"));
		});
	}

	if (feed) {
		const reader = feed.pipeThrough(new TextDecoderStream()).getReader();
		const stop = () => void reader.cancel().catch(() => {});
		server.addEventListener("close", stop);
		server.addEventListener("error", stop);
		void (async () => {
			try {
				await frames(reader, server);
			} catch {
				// The stream broke: close below.
			}
			close(1011, "hub unavailable; reconnect with after");
		})();
	}
	return new Response(null, { status: 101, webSocket: client });
}

// frames sends every line of a stream (it ends with a newline) as one text frame.
async function frames(reader, socket) {
	let buffer = "";
	for (;;) {
		const { value, done } = await reader.read();
		if (done) break;
		buffer += value;
		const complete = buffer.split("\n");
		buffer = complete.pop();
		for (const line of complete) if (line) socket.send(line);
	}
}
