// charter-ts docs --review: Claude (the claude command, Claude Code) brings docs/ up to date and into line
// with docs/charter/writing.md, with what charter-ts docs found in the prompt (review.md, the same for every
// repo). It may edit files and run the checks, nothing else.
import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { parseArgs } from "node:util";
import { cliPath, packageDir } from "./package.ts";
import { repoRoot } from "./settings.ts";

export function docsReview(args: string[]): void {
	const { values } = parseArgs({ args, options: { print: { type: "boolean" } } });
	const root = repoRoot();
	const lint = spawnSync(process.execPath, [cliPath, "docs"], { cwd: root, encoding: "utf8" });
	const prompt = readFileSync(join(packageDir, "review.md"), "utf8").replace("__LINT__", `${lint.stdout}${lint.stderr}`.trim());
	if (values.print) {
		console.log(prompt);
		return;
	}
	console.log("Claude is reviewing docs/ (it prints its report when it is done; watch `git status` for its edits)");
	const claude = spawnSync("claude", ["-p", prompt, "--permission-mode", "acceptEdits", "--allowedTools", "Read,Edit,Write,Glob,Grep,Bash(npm run docs),Bash(npm run),Bash(git status:*),Bash(git diff:*),Bash(ls:*)"], { cwd: root, stdio: ["ignore", "inherit", "inherit"] });
	if (claude.error) throw new Error("docs --review needs the claude command (Claude Code) on PATH");
	if (claude.status !== 0) throw new Error(`claude stopped (${claude.status})`);
}
