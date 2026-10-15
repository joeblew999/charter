// What a cloudflare.config.ts takes from charter (cloudflare.ts): a Rate Limiting binding for each limit
// the spec declares, counting in a namespace named from the Worker's name.
//   node --test test/*.test.ts   (npm run check -w charter-ts runs them)
import assert from "node:assert/strict";
import { it } from "node:test";
import { namespaceOf, type RateLimitOptions, rateLimits } from "../src/cloudflare.ts";

const spec = { paths: {
	"/api/notes": { get: { responses: {} }, post: { responses: {}, "x-rate-limit": { binding: "WRITE_LIMIT", limit: 100, period: 60 } } },
	"/api/files": { post: { responses: {}, "x-rate-limit": { binding: "UPLOAD_LIMIT", limit: 10, period: 10 } } },
} };

it("is a binding for each limit the spec declares, with its limit and period", () => {
	const declared = rateLimits(spec, "my-api", (options: RateLimitOptions) => options);
	assert.deepEqual(Object.keys(declared).sort(), ["UPLOAD_LIMIT", "WRITE_LIMIT"]);
	assert.deepEqual(declared.WRITE_LIMIT?.simple, { limit: 100, period: 60 });
	assert.equal(declared.WRITE_LIMIT?.namespace, namespaceOf("my-api", "WRITE_LIMIT"));
});

it("names each counter from the Worker and the binding: a positive integer, the same on every deploy, another for another Worker", () => {
	const namespace = namespaceOf("my-api", "WRITE_LIMIT");
	assert.match(namespace, /^[1-9]\d*$/);
	assert.equal(namespaceOf("my-api", "WRITE_LIMIT"), namespace);
	assert.notEqual(namespaceOf("my-api-api", "WRITE_LIMIT"), namespace);
	assert.notEqual(namespaceOf("my-api", "UPLOAD_LIMIT"), namespace);
});

it("is none for a spec with no limits", () => {
	assert.deepEqual(rateLimits({ paths: { "/api/hello": { get: { responses: {} } } } }, "my-api", options => options), {});
});
