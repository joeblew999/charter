#!/usr/bin/env node
// charter-ts: a repo kept the charter way, and a TypeScript API on Cloudflare Workers step by step,
// from its charter.config.ts. A repo's npm scripts call it (npm run docs:lint runs charter-ts docs:lint),
// so every step is the same in every repo, on every machine and on GitHub.
import { spawn } from "node:child_process";
import { version } from "./package.ts";
import { seconds } from "./scripts.ts";
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
// Every command says how long it took (on stderr, so a command's own output stays its own), when that
// is a second or more: the programs it ran say theirs as they end (scripts.ts: run).
const start = performance.now();
try {
	await step.run(args);
	if (performance.now() - start >= 1000) console.error(`charter-ts ${name} took ${seconds(performance.now() - start)}`);
} catch (error) {
	console.error(`charter-ts ${name} failed after ${seconds(performance.now() - start)}: ${error instanceof Error ? error.message : String(error)}`);
	process.exit(1);
}
