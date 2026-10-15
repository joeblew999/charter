// A webhook's receiver, as a customer of the API runs one: it takes every POST (the noteCreated webhook)
// and keeps its body and signature, for the tests to read at GET /deliveries. charter-ts runs it beside
// the server for the server's whole run (charter.config.ts: serve), so every note the tests create has
// somewhere to be delivered. Usage: node test/webhook-receiver.ts <port>
import { createServer } from "node:http";

const port = Number(process.argv[2]);
if (!port) {
	console.log("usage: node test/webhook-receiver.ts <port>");
	process.exit(1);
}
const deliveries: { body: string; signature: string }[] = [];
createServer(async (request, response) => {
	if (request.method === "POST") {
		let body = "";
		for await (const chunk of request) body += chunk;
		deliveries.push({ body, signature: String(request.headers["x-webhook-signature"] ?? "") });
		response.end("ok");
		return;
	}
	response.setHeader("content-type", "application/json");
	response.end(request.url === "/deliveries" ? JSON.stringify(deliveries) : '"ready"');
}).listen(port, "localhost");
