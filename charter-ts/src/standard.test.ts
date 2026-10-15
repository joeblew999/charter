// The standard test (standard.ts) fails on an API that does not serve the standard. That it passes on
// one that does is shown by every project's check, which runs it on the project's own server.
//   node --test src/*.test.ts   (npm run check -w charter-ts runs them)
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { after, it } from "node:test";
import { standard } from "./standard.ts";

// An API that gets the standard wrong: its spec names another address, its 404 is plain text, and it
// has no MCP endpoint.
const server = createServer((req, res) => {
	if (req.url === "/api/openapi.json") return void res.writeHead(200, { "content-type": "application/json" }).end(JSON.stringify({ servers: [{ url: "https://elsewhere.invalid" }], paths: { "/api/hello": { get: { operationId: "hello" } } } }));
	if (req.url === "/api/asyncapi.json") return void res.writeHead(200, { "content-type": "application/json" }).end("{}");
	res.writeHead(404, { "content-type": "text/plain" }).end("not found");
});
await new Promise<void>(resolve => server.listen(0, "localhost", resolve));
after(() => server.close());
const { port } = server.address() as { port: number };

it("fails, naming each thing the API gets wrong", async () => {
	await assert.rejects(standard(`http://localhost:${port}`, () => {}), (error: Error) =>
		["GET /api/openapi.json names this address", "an unknown path is 404, as problem details", "POST /api/mcp lists tools"].every(name => error.message.includes(name)));
});
