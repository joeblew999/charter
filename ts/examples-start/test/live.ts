// Live test of the API's own routes, against a local server or the deployed Worker (what every API
// must serve is charter-ts's standard test, which check and live-test run first).
// What it gets back is checked against the contract's schemas. Add a check for each route you add.
// Usage, from the project's folder: node test/live.ts <origin>
import { hello } from "../api/schemas.ts";

const origin = process.argv[2];
if (!origin) {
	console.log("FAIL  usage: node test/live.ts <origin>");
	process.exit(1);
}
let failed = 0;
function check(name: string, ok: boolean, detail: unknown): void {
	console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok ? "" : `: ${JSON.stringify(detail)}`}`);
	if (!ok) failed++;
}

const res = await fetch(`${origin}/api/hello`);
const body = hello.safeParse(await res.json().catch(() => null));
check("GET /api/hello", res.status === 200 && body.success && body.data.message.startsWith("Hello from "), { status: res.status, body });

process.exit(failed ? 1 : 0);
