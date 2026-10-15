#!/usr/bin/env node
// charter-ts: a repo kept the charter way, and a TypeScript API on Cloudflare Workers step by step,
// from its charter.config.ts. A repo's npm scripts call it, each the command of its name (npm run docs
// runs charter-ts docs),
// so every step is the same in every repo, on every machine and on GitHub.
import { spawn } from "node:child_process";
import { version } from "./package.ts";
import { seconds } from "./scripts.ts";
import { renamed, commands } from "./commands.ts";

// A Mac idle for a minute sleeps, and a Worker that cannot answer while it sleeps fails its check:
// the Mac stays awake while charter-ts runs, and no longer (caffeinate ends with this process).
if (process.platform === "darwin") spawn("caffeinate", ["-i", "-w", String(process.pid)], { stdio: "ignore", detached: true }).unref();

const [name, ...args] = process.argv.slice(2);
const step = name ? commands[name] : undefined;
if (name && renamed[name]) {
	console.error(`charter-ts ${name} is charter-ts ${renamed[name]} now (npm run adopt brings a repo's scripts to the new names)`);
	process.exit(1);
}
if (!step) {
	// What a project runs, then, with help, every command.
	const all = name === "help" || name === "--help";
	console.log(`charter-ts ${version()}: APIs on Cloudflare Workers, in TypeScript or Go, and the repos that keep them. Run each as npm run <command>.\n`);
	const shown = Object.entries(commands).filter(([, s]) => all || !s.anyRepo || s.usage.startsWith("check"));
	for (const [, s] of shown) console.log(all ? `  ${s.usage}${s.anyRepo ? "   (any repo)" : ""}\n      ${s.about}\n` : `  ${s.usage}`);
	if (!all) console.log("\ncharter-ts help: every command, and what each does");
	process.exit(name && !all ? 1 : 0);
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
