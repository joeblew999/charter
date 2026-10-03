// Who may write, by every way in, against a running Worker that trusts a test issuer this script
// serves on <keys port>: as Cloudflare Access (ACCESS_TEAM_DOMAIN=http://localhost:<keys port>,
// ACCESS_AUD=test-aud) and as an OpenID Connect issuer (OIDC_ISSUER the same, OIDC_AUDIENCE=
// https://notes.test). Under workerd this is what proves the Worker verifies a JWT. Then the rate
// limit on creating notes (x-rate-limit in the spec): a caller of its own may write that many times,
// then gets 429 with Retry-After, while another caller still writes. Never point a deployed Worker
// at it: the keys are made here, for this run.
// Usage: node test/auth-test.mjs <origin> <keys port>   (mise run test:native, test:workerd)
import { createServer } from "node:http";
import { createRequire } from "node:module";
const { exportJWK, generateKeyPair, SignJWT } = createRequire(`${process.cwd()}/`)("jose");

const [origin, keysPort] = [process.argv[2], Number(process.argv[3])];
const issuer = `http://localhost:${keysPort}`;
const { publicKey, privateKey } = await generateKeyPair("RS256");
const jwks = JSON.stringify({ keys: [{ ...(await exportJWK(publicKey)), kid: "test", alg: "RS256", use: "sig" }] });
const keys = createServer((req, res) => {
  res.writeHead(200, { "content-type": "application/json" });
  res.end(req.url === "/.well-known/openid-configuration" ? JSON.stringify({ issuer, jwks_uri: `${issuer}/jwks` }) : jwks);
});
await new Promise(resolve => keys.listen(keysPort, "localhost", resolve));

const sign = claims => new SignJWT({ iss: issuer, exp: Math.floor(Date.now() / 1000) + 300, ...claims }).setProtectedHeader({ alg: "RS256", kid: "test" }).sign(privateKey);
const access = async claims => ({ "cf-access-jwt-assertion": await sign({ aud: ["test-aud"], type: "app", ...claims }) });
const oidc = async claims => ({ authorization: `Bearer ${await sign({ aud: "https://notes.test", sub: "user-1", ...claims })}` });
const person = { email: "dev@example.com", sub: "u1" }, machine = { common_name: "ci.access", sub: "" };

let failed = 0;
try {
  for (const [name, headers, want, says] of [
    ["a person through Access writes", await access(person), 200],
    ["a machine's service token may not write", await access(machine), 403, "needs scope write"],
    ["a machine's service token with the write token writes", { ...(await access(machine)), authorization: `Bearer ${process.env.WRITE_TOKEN}` }, 200],
    ["an Access token for another application", await access({ ...person, aud: ["another"] }), 401, "refused"],
    ["an expired Access token", await access({ ...person, exp: Math.floor(Date.now() / 1000) - 600 }), 401, "refused"],
    ["an OIDC token with write writes", await oidc({ scope: "openid write" }), 200],
    ["an OIDC token with read may not write", await oidc({ scope: "openid read" }), 403, "needs scope write"],
    ["an OIDC token for another API", await oidc({ scope: "write", aud: "https://elsewhere.test" }), 401, "refused"],
    ["the read token may not write", { authorization: `Bearer ${process.env.READ_TOKEN}` }, 403],
    ["nothing", {}, 401, "credentials are required"],
  ]) {
    const res = await fetch(`${origin}/api/notes`, { method: "POST", headers: { "content-type": "application/json", ...headers }, body: JSON.stringify({ body: `auth test: ${name}` }) });
    const text = await res.text();
    const ok = res.status === want && (!says || text.includes(says));
    console.log(`${ok ? "PASS" : "FAIL"}  ${name}: ${res.status}${ok ? "" : ` ${text}`}`);
    if (!ok) failed++;
  }
  const res = await fetch(`${origin}/.well-known/openid-configuration`, { redirect: "manual" });
  const ok = res.status === 302 && res.headers.get("location") === `${issuer}/.well-known/openid-configuration`;
  console.log(`${ok ? "PASS" : "FAIL"}  the spec's openIdConnectUrl sends a client on to the issuer: ${res.status} ${res.headers.get("location")}`);
  if (!ok) failed++;

  // The rate limit, per caller: each a subject of its own, so the other tests' writes do not count.
  const { limit, period } = (await (await fetch(`${origin}/api/openapi.json`)).json()).paths["/api/notes"].post["x-rate-limit"];
  const write = async (sub, n) => fetch(`${origin}/api/notes`, { method: "POST", headers: { "content-type": "application/json", ...(await oidc({ sub, scope: "write" })) }, body: JSON.stringify({ body: `rate limit test ${n}` }) });
  const sub = `rate-limit-${Date.now()}`, statuses = [];
  for (let n = 0; n < limit; n++) statuses.push((await write(sub, n)).status);
  const over = await write(sub, limit), other = await write(`${sub}-other`, 0);
  const checks = [
    [`${limit} writes in ${period} s by one caller`, statuses.every(s => s === 200), statuses.filter(s => s !== 200).join(" ")],
    ["one more is 429, with Retry-After", over.status === 429 && over.headers.get("retry-after") === String(period), `${over.status} ${over.headers.get("retry-after")} ${await over.text()}`],
    ["another caller still writes", other.status === 200, other.status],
  ];
  for (const [name, pass, got] of checks) {
    console.log(`${pass ? "PASS" : "FAIL"}  ${name}${pass ? "" : `: ${got}`}`);
    if (!pass) failed++;
  }
} finally {
  keys.close();
}
process.exit(failed ? 1 : 0);
