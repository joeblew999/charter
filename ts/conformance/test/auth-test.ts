// Who may do what, by every way in, against a running conformance server (go/conformance or this one)
// that trusts a test issuer this script serves on <keys port>: as Cloudflare Access
// (ACCESS_TEAM_DOMAIN=http://localhost:<keys port>, ACCESS_AUD=test-aud) and as an OpenID Connect
// issuer (OIDC_ISSUER the same, OIDC_AUDIENCE=https://conformance.test); beside them, its OAuth client
// credentials and its own tokens (READ_TOKEN and WRITE_TOKEN set). Under workerd this is what proves
// the server verifies a JWT. Then the rate limit on creating notes (writeLimit in the contract, the same
// in Go): a caller of its own may write that many times, then gets 429 with Retry-After, while another
// caller still writes. Never point a deployed server at it: the keys are made here, for this run.
// Usage: node test/auth-test.ts <api> <keys port>   (the origin of the Go server, <origin>/api/mock of the oRPC one)
import { createServer } from "node:http";
import { exportJWK, generateKeyPair, type JWTPayload, SignJWT } from "jose";
import { writeLimit } from "../src/contract.ts";

const api = process.argv[2], keysPort = Number(process.argv[3]);
const { READ_TOKEN, WRITE_TOKEN } = process.env;
if (!api || !keysPort || !READ_TOKEN || !WRITE_TOKEN) {
	console.log("FAIL  usage: node test/auth-test.ts <api> <keys port>, with READ_TOKEN and WRITE_TOKEN set");
	process.exit(1);
}
const issuer = `http://localhost:${keysPort}`;
const { publicKey, privateKey } = await generateKeyPair("RS256");
const jwks = JSON.stringify({ keys: [{ ...(await exportJWK(publicKey)), kid: "test", alg: "RS256", use: "sig" }] });
const keys = createServer((req, res) => {
	res.writeHead(200, { "content-type": "application/json" });
	res.end(req.url === "/.well-known/openid-configuration" ? JSON.stringify({ issuer, jwks_uri: `${issuer}/jwks` }) : jwks);
});
await new Promise<void>(resolve => keys.listen(keysPort, "localhost", () => resolve()));

type Headers = Record<string, string>;
const sign = (claims: JWTPayload) => new SignJWT({ iss: issuer, exp: Math.floor(Date.now() / 1000) + 300, ...claims }).setProtectedHeader({ alg: "RS256", kid: "test" }).sign(privateKey);
const access = async (claims: JWTPayload): Promise<Headers> => ({ "cf-access-jwt-assertion": await sign({ aud: ["test-aud"], type: "app", ...claims }) });
const oidc = async (claims: JWTPayload): Promise<Headers> => ({ authorization: `Bearer ${await sign({ aud: "https://conformance.test", sub: "user-1", ...claims })}` });
const bearer = (token: string): Headers => ({ authorization: `Bearer ${token}` });
const person = { email: "dev@example.com", sub: "u1" }, machine = { common_name: "ci.access", sub: "" };

let failed = 0;
const report = (name: string, ok: boolean, why: string) => {
	console.log(`${ok ? "PASS" : "FAIL"}  ${name}${why}`);
	if (!ok) failed++;
};
try {
	// OAuth client credentials, as the SDKs do it.
	const issued = await fetch(`${api}/oauth/token`, { method: "POST", body: new URLSearchParams({ client_id: "id-1", client_secret: "secret-1" }) });
	const { access_token } = (await issued.json()) as { access_token: string };

	const writes: [name: string, headers: Headers, want: number, says?: string][] = [
		["an OAuth token writes", bearer(access_token), 200],
		["the write token writes", bearer(WRITE_TOKEN), 200],
		["the read token may not write", bearer(READ_TOKEN), 403, "needs scope write"],
		["a person through Access writes", await access(person), 200],
		["a machine's service token may not write", await access(machine), 403, "needs scope write"],
		["a machine's service token with the write token writes", { ...(await access(machine)), ...bearer(WRITE_TOKEN) }, 200],
		["an Access token for another application", await access({ ...person, aud: ["another"] }), 401, "refused"],
		["an expired Access token", await access({ ...person, exp: Math.floor(Date.now() / 1000) - 600 }), 401, "refused"],
		["an OIDC token with write writes", await oidc({ scope: "openid write" }), 200],
		["an OIDC token with read may not write", await oidc({ scope: "openid read" }), 403, "needs scope write"],
		["an OIDC token for another API", await oidc({ scope: "write", aud: "https://elsewhere.test" }), 401, "refused"],
		["a token no one issued", bearer("never-issued"), 401, "refused"],
		["nothing", {}, 401, "credentials are required"],
	];
	for (const [name, headers, want, says] of writes) {
		const res = await fetch(`${api}/notes`, { method: "POST", headers: { "content-type": "application/json", ...headers }, body: JSON.stringify({ body: `auth test: ${name}` }) });
		const text = await res.text();
		const ok = res.status === want && (!says || text.includes(says));
		report(`create: ${name}`, ok, `: ${res.status}${ok ? "" : ` ${text}`}`);
	}
	const reads: [name: string, headers: Headers, want: number][] = [
		["the read token reads", bearer(READ_TOKEN), 200],
		["a machine's service token reads", await access(machine), 200],
		["an OIDC token with read reads", await oidc({ scope: "read" }), 200],
		["nothing", {}, 401],
	];
	for (const [name, headers, want] of reads) {
		const res = await fetch(`${api}/notes`, { headers });
		report(`list: ${name}`, res.status === want, `: ${res.status}${res.status === want ? "" : ` ${await res.text()}`}`);
	}
	const res = await fetch(`${api}/.well-known/openid-configuration`, { redirect: "manual" });
	report("the spec's openIdConnectUrl sends a client on to the issuer", res.status === 302 && res.headers.get("location") === `${issuer}/.well-known/openid-configuration`, `: ${res.status} ${res.headers.get("location")}`);

	// The rate limit, per caller: each a subject of its own, so the other tests' writes do not count.
	const { limit, period } = writeLimit;
	const write = async (sub: string, n: number) => fetch(`${api}/notes`, { method: "POST", headers: { "content-type": "application/json", ...(await oidc({ sub, scope: "write" })) }, body: JSON.stringify({ body: `rate limit test ${n}` }) });
	// The window is fixed: writes that cross its end start a new count. So write until the first
	// refusal, at most two windows' worth: before it, at least limit writes went through, and none was
	// refused.
	const sub = `rate-limit-${Date.now()}`;
	let through = 0, over: Response | undefined;
	for (let n = 0; n <= 2 * limit; n++) {
		const res = await write(sub, n);
		if (res.status !== 200) { over = res; break; }
		through++;
	}
	const other = await write(`${sub}-other`, 0);
	report(`at least ${limit} writes in ${period} s by one caller`, through >= limit, `: ${through} went through`);
	report("then a 429, with Retry-After", over?.status === 429 && over.headers.get("retry-after") === String(period), over ? `: ${over.status} ${over.headers.get("retry-after")} ${await over.text()}` : `: none in ${through} writes`);
	report("another caller still writes", other.status === 200, `: ${other.status}`);
} finally {
	keys.close();
}
process.exit(failed ? 1 : 0);
