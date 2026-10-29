// The rate limit on Cloudflare (Workers Rate Limiting), against the deployed Worker: one caller (the
// write token) writes until the limit refuses it, which must be a 429 with Retry-After, while another
// caller (a token from the API's own OAuth endpoint) still writes. Locally, test/auth.ts checks it
// exactly; on Cloudflare the limit counts per location and is eventually consistent, and the other
// live tests write with the same token, so this checks the refusal, not the count.
// Usage: node test/ratelimit.ts <origin>, with WRITE_TOKEN set (fnox exec --). It writes test notes.
import { writeLimit } from "../api/contract.ts";

const origin = process.argv[2];
const { WRITE_TOKEN } = process.env;
if (!origin || !WRITE_TOKEN) {
	console.log("FAIL  usage: node test/ratelimit.ts <origin>, with WRITE_TOKEN set (fnox exec --)");
	process.exit(1);
}
const write = (authorization: string, n: number) =>
	fetch(`${origin}/api/notes`, { method: "POST", headers: { "content-type": "application/json", authorization }, body: JSON.stringify({ body: `rate limit test ${n}` }) });

// The other caller: a token the API issues for its OAuth client (the conformance API's defaults).
const token = await fetch(`${origin}/api/oauth/token`, {
	method: "POST",
	headers: { "content-type": "application/x-www-form-urlencoded" },
	body: new URLSearchParams({ grant_type: "client_credentials", client_id: "id-1", client_secret: "secret-1" }),
});
const { access_token } = (await token.json().catch(() => ({}))) as { access_token?: string };

const { limit, period } = writeLimit;
let through = 0, over: Response | undefined;
for (let n = 0; n < 3 * limit; n++) {
	const res = await write(`Bearer ${WRITE_TOKEN}`, n);
	if (res.status !== 200) {
		over = res;
		break;
	}
	through++;
}
const other = access_token ? await write(`Bearer ${access_token}`, 0) : undefined;

let failed = 0;
const report = (name: string, ok: boolean, detail: string) => {
	console.log(`${ok ? "PASS" : "FAIL"}  ${name}${detail}`);
	if (!ok) failed++;
};
report("the write token gets a 429 with Retry-After, within three windows' worth of writes", over?.status === 429 && over.headers.get("retry-after") === String(period), over ? `: after ${through} writes, ${over.status} ${over.headers.get("retry-after")}` : `: none in ${through} writes`);
report("another caller (an OAuth token) still writes", other?.status === 200, other ? `: ${other.status}` : `: no OAuth token (${token.status})`);
process.exit(failed ? 1 : 0);
