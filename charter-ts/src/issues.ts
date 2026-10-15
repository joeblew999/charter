// charter-ts issues: the repo's open issues, newest first (what is reported, and what is planned: a
// plan is an issue); with a kind, the body of a new one with the headings of that issue form (charter's
// .github/ISSUE_TEMPLATE/charter-<kind>.yml) and the gh command that files it: gh and the API ignore forms.
import { existsSync, readFileSync } from "node:fs";
import { charterFile } from "./charter-files.ts";
import { run } from "./run.ts";

export function issues(args: string[]): void {
	const [kind] = args;
	if (!kind) return run("gh", ["issue", "list", "--state", "open", "--limit", "100"]);
	const form = charterFile(`.github/ISSUE_TEMPLATE/charter-${kind}.yml`);
	if (/[/.]/.test(kind) || !existsSync(form)) throw new Error(`issues takes a kind of issue: bug, feature, upstream or plan, not ${kind}`);
	const lines = readFileSync(form, "utf8").split("\n");
	let title = "", labels = "";
	const body: string[] = [];
	lines.forEach((line, i) => {
		const text = line.trim();
		if (line.startsWith("title: ")) title = line.slice("title: ".length).replace(/^"|"$/g, "");
		else if (line.startsWith("labels: ")) labels = line.slice("labels: ".length).replace(/[[\]"]/g, "").replaceAll(", ", ",");
		else if (text.startsWith("label: ")) {
			body.push(`### ${text.slice("label: ".length)}`, "");
			const hint = lines[i + 1]?.trim();
			if (hint?.startsWith("description: ")) body.push(`<!-- ${hint.slice("description: ".length)} -->`, "");
			body.push("", "");
		}
	});
	console.log(`<!-- Fill this in, then: gh issue create --title ${JSON.stringify(`${title}...`)} --label ${labels},agent-filed --body-file <this file> -->\n`);
	console.log(body.join("\n"));
}
