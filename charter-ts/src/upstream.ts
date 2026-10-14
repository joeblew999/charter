// charter-ts upstream:status: the upstream issues the code works around, and their state. A tag is a
// comment line such as  Upstream: owner/repo#123 (when fixed: what to do then). CLOSED means that
// workaround can go: the tag says what to do then.
import { spawnSync } from "node:child_process";
import { output } from "./scripts.ts";
import { repoRoot } from "./settings.ts";

const tag = /^([^:]+:[0-9]+):.*Upstream: ([\w.-]+\/[\w.-]+)#([0-9]+) *(.*)$/;

export function upstreamStatus(): void {
	// Not where a tag is written about: the docs, the task files, and the code that reads tags.
	const grep = spawnSync("git", ["grep", "-n", "-E", "Upstream: [A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+#[0-9]+", "--", ":!*.md", ":!*mise.toml", ":!*tasks.toml", ":!*upstream.ts", ":!*upstream-status.go"], { cwd: repoRoot(), encoding: "utf8" });
	if (grep.status === 1) {
		console.log("0 upstream issues: no Upstream: tag in the code here");
		return;
	}
	if (grep.status !== 0) throw new Error(`git grep failed: ${grep.stderr}`);
	const places = new Map<string, string[]>();
	for (const line of grep.stdout.split("\n")) {
		const m = tag.exec(line);
		if (!m) continue;
		const issue = `${m[2]}#${m[3]}`;
		places.set(issue, [...(places.get(issue) ?? []), `        ${m[1]}  ${m[4]}`]);
	}
	let closed = 0;
	for (const issue of [...places.keys()].sort()) {
		const [repo, number] = issue.split("#");
		let state = "UNREACHABLE", title = "";
		try {
			[state, title] = output("gh", ["api", `repos/${repo}/issues/${number}`, "--jq", '.state + "\\t" + .title']).trim().split("\t");
			state = state.toUpperCase();
		} catch {
			// gh could not reach it
		}
		if (state === "CLOSED") closed++;
		console.log(`${state}  ${issue}  ${title}\n${places.get(issue)!.join("\n")}`);
	}
	console.log(`\n${places.size} upstream issues, ${closed} closed${closed > 0 ? ": remove those workarounds (each tag says what to do then)" : ""}`);
}
