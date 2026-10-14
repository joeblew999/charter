// Live test of the API: what a deploy must pass, run against a local server or the deployed Worker.
// Add a check for each route you add. Usage, from the project's folder: node test/live-test.mjs <origin>
const origin = process.argv[2];
let failed = 0;
function check(name, ok, detail) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok ? "" : `: ${JSON.stringify(detail)}`}`);
  if (!ok) failed++;
}

const hello = await fetch(`${origin}/api/hello`);
const body = await hello.json().catch(() => null);
check("GET /api/hello", hello.status === 200 && body?.message?.startsWith("Hello from "), { status: hello.status, body });

const spec = await fetch(`${origin}/api/openapi.json`);
const openapi = await spec.json().catch(() => null);
check("GET /api/openapi.json names this origin", spec.status === 200 && openapi?.servers?.[0]?.url === origin, { status: spec.status, servers: openapi?.servers });

process.exit(failed ? 1 : 0);
