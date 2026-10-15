// npm pack (prepack): the examples a new project is copied from, into templates/ of the package, each
// with the workspace's allowScripts (npm reads them only at a root: a new project is one).
import { readFileSync, rmSync, writeFileSync } from "node:fs";
import { copyProject, templates } from "./src/new.ts";

const { allowScripts } = JSON.parse(readFileSync("../package.json", "utf8")) as { allowScripts?: Record<string, boolean> };
rmSync("templates", { recursive: true, force: true });
for (const template of templates) {
	copyProject(`../ts/examples-${template}`, `templates/${template}`, "gitignore");
	const file = `templates/${template}/package.json`;
	writeFileSync(file, JSON.stringify({ ...JSON.parse(readFileSync(file, "utf8")), allowScripts }, null, "\t") + "\n");
}
console.log(`templates/: ${templates.join(", ")}`);
