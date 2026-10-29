#!/usr/bin/env node
// charter-ts, the command: runs one of its commands (commands.ts) and says how long it took. A repo's
// npm scripts call it, each script the command of its name: npm run check runs charter-ts check.
import { commands, renamed, renamedTo } from "./commands.ts";
import { version } from "./package.ts";
import { seconds } from "./run.ts";

const [name, ...args] = process.argv.slice(2);

if (name === undefined || name === "help" || name === "--help") {
	help(name !== undefined);
	process.exit(0);
}
if (renamed[name]) fail(`charter-ts ${name} is ${renamedTo(name)} now (npm run adopt brings a repo's scripts to the new names)`);
const command = commands[name];
if (!command) fail(`charter-ts has no command ${name}: charter-ts help lists them`);

const start = performance.now();
try {
	await command.run(args);
	// On stderr, so a command's own output stays its own. The programs it ran said their times as they ended.
	if (performance.now() - start >= 1000) console.error(`charter-ts ${name} took ${seconds(performance.now() - start)}`);
} catch (error) {
	fail(`charter-ts ${name} failed after ${seconds(performance.now() - start)}: ${error instanceof Error ? error.message : String(error)}`);
}

/** The commands; with all, where each works and what it does. A project's steps are charter's. */
function help(all: boolean): void {
	console.log(`charter-ts ${version()}: a repo kept the charter way. Run a command as npm run <command>; a project's steps are charter's (charter.toml: charter help).\n`);
	for (const c of Object.values(commands)) {
		if (all) console.log(`  ${c.usage}   (${c.where === "both" ? "in a project or any repo" : "in any repo"})\n      ${c.about}\n`);
		else console.log(`  ${c.usage}`);
	}
	if (!all) console.log("\ncharter-ts help: every command, and what each does");
}

function fail(message: string): never {
	console.error(message);
	process.exit(1);
}
