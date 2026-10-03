import { describe, expect, it } from "vitest";
import { oc } from "@orpc/contract";
import { openapi } from "@orpc/openapi";
import { z } from "zod";
import { authorize, needs, open, scheme, securityOf } from "../src/auth.ts";
import { openapiSpec } from "../src/specs.ts";

// A public hello, a list that takes the API's default (read), and a create that needs write.
const contract = {
	hello: oc.meta(openapi({ method: "GET", path: "/hello", spec: open() })).output(z.string()),
	list: oc.meta(openapi({ method: "GET", path: "/notes" })).output(z.string()),
	create: oc.meta(openapi({ method: "POST", path: "/notes", spec: needs(["write"]) })).output(z.string()),
};
const defaults = scheme("read").security;
const tokens = [
	{ value: "w", scopes: ["read", "write"] },
	{ value: "r", scopes: ["read"] },
];
const status = (procedure: unknown, token?: string) =>
	authorize(token === undefined ? null : `Bearer ${token}`, securityOf(procedure, defaults), tokens)?.status ?? 200;

describe("auth", () => {
	it("enforces each operation's scopes", () => {
		expect(status(contract.hello)).toBe(200);
		expect(status(contract.list)).toBe(401);
		expect(status(contract.list, "wrong")).toBe(401);
		expect(status(contract.list, "r")).toBe(200);
		expect(status(contract.list, "w")).toBe(200);
		expect(status(contract.create)).toBe(401);
		expect(status(contract.create, "r")).toBe(403);
		expect(status(contract.create, "w")).toBe(200);
	});

	it("matches no token when a secret is unset", () => {
		expect(authorize("Bearer ", securityOf(contract.list, defaults), [{ value: undefined, scopes: ["read"] }])?.status).toBe(401);
		expect(authorize("Bearer ", securityOf(contract.list, defaults), [{ value: "", scopes: ["read"] }])?.status).toBe(401);
	});

	it("puts what it enforces in the spec", async () => {
		const doc = await openapiSpec(contract, { info: { title: "t", version: "1" }, server: "https://x.test", base: scheme("read") });
		expect(doc.components?.securitySchemes).toEqual({ bearer: { type: "http", scheme: "bearer" } });
		expect(doc.security).toEqual([{ bearer: ["read"] }]);
		expect(doc.paths?.["/hello"]?.get?.security).toEqual([]);
		expect(doc.paths?.["/notes"]?.post?.security).toEqual([{ bearer: ["write"] }]);
	});
});
