import { describe, expect, it } from "vitest";
import { oc } from "@orpc/contract";
import { openapi } from "@orpc/openapi";
import { z } from "zod";
import { needs, open, scheme } from "../src/auth.ts";
import { keyOf, limited, limitOf, rateLimit, type Limit, type RateLimiter } from "../src/ratelimit.ts";
import { openapiSpec } from "../src/specs.ts";

const writes: Limit = { binding: "WRITE_LIMIT", limit: 2, period: 60, scope: "write" };
const hellos: Limit = { binding: "HELLO_LIMIT", limit: 3, period: 10 };

// A public hello with a limit of its own, a list with none, and a create limited by the write scope's.
const contract = {
	hello: oc.meta(openapi({ method: "GET", path: "/hello", spec: open(limited(hellos)) })).output(z.string()),
	list: oc.meta(openapi({ method: "GET", path: "/notes", spec: open() })).output(z.string()),
	create: oc.meta(openapi({ method: "POST", path: "/notes", spec: needs(["write"]) })).output(z.string()),
};

// A binding's fixed window, in memory, as workerd's local one.
const binding = (): RateLimiter => {
	const counts = new Map<string, number>();
	return { limit: async ({ key }) => { counts.set(key, (counts.get(key) ?? 0) + 1); return { success: counts.get(key)! <= 2 }; } };
};

describe("ratelimit", () => {
	it("finds each procedure's limit: its own, its scope's, or none", () => {
		expect(limitOf(contract.hello, [writes])).toEqual(hellos);
		expect(limitOf(contract.create, [writes])).toEqual(writes);
		expect(limitOf(contract.create)).toBeUndefined();
		expect(limitOf(contract.list, [writes])).toBeUndefined();
	});

	it("puts each limit and its 429 in the spec", async () => {
		const doc = await openapiSpec(contract, { info: { title: "t", version: "1" }, server: "http://x", base: scheme(), rateLimits: [writes] }) as any;
		expect(doc.paths["/hello"].get["x-rate-limit"]).toEqual(hellos);
		expect(doc.paths["/notes"].post["x-rate-limit"]).toEqual(writes);
		expect(doc.paths["/notes"].get["x-rate-limit"]).toBeUndefined();
		for (const op of [doc.paths["/hello"].get, doc.paths["/notes"].post]) {
			expect(op.responses["429"].headers["Retry-After"].schema).toEqual({ type: "integer" });
		}
		expect(doc.paths["/notes"].get.responses["429"]).toBeUndefined();
	});

	it("keys on the caller, else the client's IP", () => {
		const h = new Headers({ "cf-connecting-ip": "203.0.113.7" });
		expect(keyOf({ scheme: "access", machine: "ci.access", subject: "s", scopes: [] }, h)).toBe("machine:ci.access");
		expect(keyOf({ scheme: "oidc", subject: "user-1", email: "a@b.c", scopes: [] }, h)).toBe("oidc:user-1");
		expect(keyOf({ scheme: "access", email: "a@b.c", scopes: [] }, h)).toBe("email:a@b.c");
		expect(keyOf({ scheme: "bearer", token: "WRITE_TOKEN", scopes: ["write"] }, h)).toBe("token:WRITE_TOKEN");
		expect(keyOf(undefined, h)).toBe("ip:203.0.113.7");
		expect(keyOf(undefined, new Headers())).toBe("ip:unknown");
	});

	it("refuses with 429 and Retry-After over the limit, per key", async () => {
		const env = { WRITE_LIMIT: binding() };
		expect((await rateLimit(writes, "a", env)).status).toBe(200);
		expect((await rateLimit(writes, "a", env)).status).toBe(200);
		expect(await rateLimit(writes, "a", env)).toMatchObject({ status: 429, retryAfter: 60 });
		expect((await rateLimit(writes, "b", env)).status).toBe(200);
		expect((await rateLimit(undefined, "a", env)).status).toBe(200);
	});

	it("lets calls through without the binding, or when it fails", async () => {
		expect((await rateLimit(writes, "a", {})).status).toBe(200);
		expect((await rateLimit(writes, "a", { WRITE_LIMIT: { limit: () => Promise.reject(new Error("down")) } })).status).toBe(200);
	});
});
