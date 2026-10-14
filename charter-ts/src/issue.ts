// charter-ts issue <kind>: an issue body with the headings of that issue form (the kit's
// .github/ISSUE_TEMPLATE/charter-<kind>.yml), for gh issue create --body-file: gh and the API ignore forms.
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { kitDir } from "./kit.ts";

export function issue(args: string[]): void {
	const [kind] = args;
	const form = join(kitDir, ".github", "ISSUE_TEMPLATE", `charter-${kind}.yml`);
	if (!kind || /[/.]/.test(kind) || !existsSync(form)) throw new Error("issue needs the kind: bug, feature, upstream or plan");
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
