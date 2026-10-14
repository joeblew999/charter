// Live test of the API: what a deploy must pass, run against a local server or the deployed Worker.
// What it gets back is checked against the contract's schemas. Add a check for each route you add.
// Usage, from the project's folder: node test/live-test.ts <origin>
import { hello } from "../src/contract.ts";

const origin = process.argv[2];
if (!origin) {
	console.log("FAIL  usage: node test/live-test.ts <origin>");
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

const spec = await fetch(`${origin}/api/openapi.json`);
const openapi = (await spec.json().catch(() => null)) as { servers?: { url: string }[] } | null;
check("GET /api/openapi.json names this origin", spec.status === 200 && openapi?.servers?.[0]?.url === origin, { status: spec.status, servers: openapi?.servers });

process.exit(failed ? 1 : 0);
