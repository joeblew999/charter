import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { createServer, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import { oc } from "@orpc/contract";
import { openapi } from "@orpc/openapi";
import { exportJWK, generateKeyPair, SignJWT, type JWTPayload } from "jose";
import { z } from "zod";
import { accessScheme, authorize, needs, oidcScheme, open, scheme, securityOf, type Trusted } from "../src/auth.ts";
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
const decide = (procedure: unknown, headers: Record<string, string>, trusted: Trusted = { tokens }) =>
	authorize(new Headers(headers), securityOf(procedure, defaults), trusted);
const status = async (procedure: unknown, token?: string) =>
	(await decide(procedure, token === undefined ? {} : { authorization: `Bearer ${token}` })).status;

// A test issuer: its keys served as Access serves them and as an OIDC issuer does, tokens signed by jose.
type Key = Awaited<ReturnType<typeof generateKeyPair>>["privateKey"];
let server: Server, issuer: string, sign: (claims: JWTPayload, options?: { alg?: string; kid?: string; key?: Key }) => Promise<string>;
let other: Key;
beforeAll(async () => {
	const { publicKey, privateKey } = await generateKeyPair("RS256");
	other = (await generateKeyPair("RS256")).privateKey;
	const jwks = JSON.stringify({ keys: [{ ...(await exportJWK(publicKey)), kid: "test", alg: "RS256", use: "sig" }] });
	server = createServer((req, res) => {
		res.setHeader("content-type", "application/json");
		res.end(req.url === "/.well-known/openid-configuration" ? JSON.stringify({ issuer, jwks_uri: `${issuer}/jwks` }) : jwks);
	});
	await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
	issuer = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
	sign = (claims, { alg = "RS256", kid = "test", key = privateKey } = {}) =>
		new SignJWT({ iss: issuer, exp: Math.floor(Date.now() / 1000) + 300, ...claims }).setProtectedHeader({ alg, kid }).sign(key);
});
afterAll(() => server.close());

describe("auth", () => {
	it("enforces each operation's scopes", async () => {
		expect(await status(contract.hello)).toBe(200);
		expect(await status(contract.list)).toBe(401);
		expect(await status(contract.list, "wrong")).toBe(401);
		expect(await status(contract.list, "r")).toBe(200);
		expect(await status(contract.list, "w")).toBe(200);
		expect(await status(contract.create)).toBe(401);
		expect(await status(contract.create, "r")).toBe(403);
		expect(await status(contract.create, "w")).toBe(200);
	});

	it("matches no token when a secret is unset", async () => {
		for (const value of [undefined, ""]) {
			expect((await decide(contract.list, { authorization: "Bearer " }, { tokens: [{ value, scopes: ["read"] }] })).status).toBe(401);
		}
	});

	it("takes Access and an OIDC issuer, with the same scopes", async () => {
		const trusted: Trusted = {
			tokens: [{ value: "w", scopes: ["read", "write"] }],
			access: { team: issuer, aud: "app-aud", people: ["read"], machines: ["read", "write"] },
			oidc: { issuer, audience: "https://notes.test" },
		};
		const access = async (claims: JWTPayload) => ({ "cf-access-jwt-assertion": await sign({ aud: ["app-aud"], ...claims }) });
		const oidc = async (claims: JWTPayload) => ({ authorization: `Bearer ${await sign({ aud: "https://notes.test", sub: "user-1", ...claims })}` });
		const cases: [string, Record<string, string>, number, string?][] = [
			["a machine writes", await access({ common_name: "ci.access", sub: "" }), 200],
			["an OIDC token with write", await oidc({ scope: "openid write" }), 200],
			["an OIDC token with read", await oidc({ scope: "read" }), 403, "needs scope write"],
			["an Access token for another application", await access({ email: "dev@example.com", aud: ["other"] }), 401, "aud"],
			["an Access token that names no one", await access({ sub: "x" }), 401, "names no one"],
			["an OIDC token for another API", await oidc({ scope: "write", aud: "https://elsewhere.test" }), 401, "aud"],
			["the bearer token beside them", { authorization: "Bearer w" }, 200],
		];
		for (const [name, headers, want, says] of cases) {
			const got = await decide(contract.create, headers, trusted);
			expect({ name, status: got.status }).toEqual({ name, status: want });
			if (says) expect("message" in got && got.message).toContain(says);
		}
		expect((await decide(contract.list, await access({ email: "dev@example.com", sub: "u1" }), trusted)).status).toBe(200);
		const person = await decide(contract.create, await access({ email: "dev@example.com", sub: "u1" }), trusted);
		expect(person).toMatchObject({ status: 403, message: "dev@example.com may not do this: it needs scope write" });
		const machine = await decide(contract.create, await access({ common_name: "ci.access", sub: "" }), trusted);
		expect(machine).toMatchObject({ status: 200, caller: { scheme: "access", machine: "ci.access" } });
		// Unset, Access and OIDC trust no one.
		const bare: Trusted = { tokens, access: { team: undefined, aud: undefined, people: ["read"], machines: [] }, oidc: { issuer: undefined, audience: undefined } };
		expect((await decide(contract.list, await access({ email: "dev@example.com" }), bare)).status).toBe(401);
		expect((await decide(contract.list, await oidc({ scope: "read" }), bare)).status).toBe(401);
	});

	it("refuses what must not verify", async () => {
		const trusted: Trusted = { oidc: { issuer, audience: "https://notes.test" } };
		const now = Math.floor(Date.now() / 1000);
		const good = await sign({ aud: "https://notes.test", sub: "u", scope: "read" });
		const [header, payload] = good.split(".");
		const b64 = (s: string) => Buffer.from(s).toString("base64url");
		const refused: Record<string, string> = {
			expired: await sign({ aud: "https://notes.test", sub: "u", exp: now - 120 }),
			"not yet valid": await sign({ aud: "https://notes.test", sub: "u", nbf: now + 120 }),
			"without exp": await sign({ aud: "https://notes.test", sub: "u", exp: undefined }),
			"for another audience": await sign({ aud: "https://elsewhere.test", sub: "u" }),
			"from another issuer": await sign({ iss: "https://evil.test", aud: "https://notes.test", sub: "u" }),
			"alg none": `${b64(JSON.stringify({ alg: "none", kid: "test" }))}.${payload}.`,
			"HS256": await new SignJWT({ iss: issuer, aud: "https://notes.test", sub: "u", exp: now + 60 }).setProtectedHeader({ alg: "HS256", kid: "test" }).sign(new TextEncoder().encode("a shared secret anyone could guess")),
			"an unknown kid": await sign({ aud: "https://notes.test", sub: "u" }, { kid: "nope", key: other }),
			"another key under its kid": await sign({ aud: "https://notes.test", sub: "u" }, { key: other }),
			"a changed claim": `${header}.${b64(JSON.stringify({ iss: issuer, aud: "https://notes.test", sub: "admin", exp: now + 60 }))}.${good.split(".")[2]}`,
			"a changed signature": `${header}.${payload}.${b64("not the signature")}`,
		};
		for (const [name, token] of Object.entries(refused)) {
			const got = await decide(contract.list, { authorization: `Bearer ${token}` }, trusted);
			expect({ name, status: got.status }).toEqual({ name, status: 401 });
		}
		expect((await decide(contract.list, { authorization: `Bearer ${good}` }, trusted)).status).toBe(200);
	});

	it("puts what it enforces in the spec, by every scheme", async () => {
		const doc = await openapiSpec(contract, { info: { title: "t", version: "1" }, server: "https://x.test", base: oidcScheme(accessScheme("notes-test", scheme("read"))) });
		expect(Object.keys(doc.components?.securitySchemes ?? {})).toEqual(["bearer", "accessClientId", "accessClientSecret", "oidc"]);
		expect(doc.components?.securitySchemes?.accessClientId).toMatchObject({ type: "apiKey", in: "header", name: "CF-Access-Client-Id", "x-fern-header": { env: "NOTES_TEST_ACCESS_CLIENT_ID" } });
		expect(doc.security).toEqual([{ bearer: ["read"] }, { accessClientId: ["read"], accessClientSecret: [] }, { oidc: ["read"] }]);
		expect(doc.paths?.["/hello"]?.get?.security).toEqual([]);
		expect(doc.paths?.["/notes"]?.post?.security).toEqual([{ bearer: ["write"] }, { accessClientId: ["write"], accessClientSecret: [] }, { oidc: ["write"] }]);
		const bearerOnly = await openapiSpec(contract, { info: { title: "t", version: "1" }, server: "https://x.test", base: scheme("read") });
		expect(bearerOnly.components?.securitySchemes).toEqual({ bearer: { type: "http", scheme: "bearer" } });
		expect(bearerOnly.paths?.["/notes"]?.post?.security).toEqual([{ bearer: ["write"] }]);
	});
});
