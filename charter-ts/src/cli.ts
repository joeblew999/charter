#!/usr/bin/env node
// charter-ts: a repo kept the charter way, and a TypeScript API on Cloudflare Workers step by step,
// from its charter.config.ts. A repo's npm scripts call it (npm run docs:lint runs charter-ts docs:lint),
// so every step is the same in every repo, on every machine and on GitHub.
import { spawn } from "node:child_process";
import { version } from "./package.ts";
import { steps } from "./steps.ts";

// A Mac idle for a minute sleeps, and a Worker that cannot answer while it sleeps fails its check:
// the Mac stays awake while charter-ts runs, and no longer (caffeinate ends with this process).
if (process.platform === "darwin") spawn("caffeinate", ["-i", "-w", String(process.pid)], { stdio: "ignore", detached: true }).unref();

const [name, ...args] = process.argv.slice(2);
const step = name ? steps[name] : undefined;
if (!step) {
	console.log(`charter-ts ${version()}: a repo kept the charter way, and a TypeScript API on Cloudflare Workers\n`);
	for (const s of Object.values(steps)) console.log(`  charter-ts ${s.usage}${s.anyRepo ? "   (any repo)" : ""}\n      ${s.about}`);
	process.exit(name ? 1 : 0);
}
try {
	await step.run(args);
} catch (error) {
	console.error(`charter-ts ${name}: ${error instanceof Error ? error.message : String(error)}`);
	process.exit(1);
}
