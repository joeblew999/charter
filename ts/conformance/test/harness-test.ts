// The SDK test the Worker runs inside itself (/harness/sdk-test, /harness/ws-test), read and reported here.
// Usage: node test/harness-test.ts <worker origin> [<target>]. target is another Worker that serves
// the API, for the WebSocket test on Cloudflare, where a Worker can't call its own URL (error 1042).
import type { Report } from "../src/report.ts";

const origin = process.argv[2], target = process.argv[3];
if (!origin) {
	console.log("FAIL  usage: node test/harness-test.ts <worker origin> [<target>]");
	process.exit(1);
}
let failed = false;
for (const path of ["/harness/sdk-test", `/harness/ws-test${target ? `?target=${target}` : ""}`]) {
	const text = await (await fetch(origin + path)).text();
	let report: Report;
	try {
		report = JSON.parse(text) as Report;
	} catch {
		console.log(`FAIL  ${path}: ${text.slice(0, 500)}`);
		failed = true;
		continue;
	}
	for (const name of Object.keys(report.results).sort()) {
		const { ok, detail } = report.results[name];
		console.log(ok ? `PASS  ${name}` : `FAIL  ${name}  ${JSON.stringify(detail).slice(0, 300)}`);
	}
	failed ||= !report.passed;
}
process.exit(failed ? 1 : 0);
