// npm pack (prepack): what the package carries from charter's repo, as it is there. The examples a new
// project is copied from, in TypeScript and in Go, into templates/, each with the workspace's
// allowScripts (npm reads them only at a root: a new project is one); and charter's files, into
// charter-files/, which adopt writes.
import { cpSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { charterFiles } from "./src/charter-files.ts";
import { copyTemplate, languages, packedName, templates } from "./src/new.ts";

const { allowScripts } = JSON.parse(readFileSync("../package.json", "utf8")) as { allowScripts?: Record<string, boolean> };
rmSync("templates", { recursive: true, force: true });
const packed = templates.flatMap(template => languages.map(language => ({ template, language, name: packedName(template, language) })));
for (const { template, language, name } of packed) {
	copyTemplate("..", template, language, `templates/${name}`, "gitignore");
	const file = `templates/${name}/package.json`;
	writeFileSync(file, JSON.stringify({ ...JSON.parse(readFileSync(file, "utf8")), allowScripts }, null, "\t") + "\n");
}
console.log(`templates/: ${packed.map(p => p.name).join(", ")}`);

rmSync("charter-files", { recursive: true, force: true });
for (const rel of charterFiles) {
	mkdirSync(dirname(join("charter-files", rel)), { recursive: true });
	cpSync(join("..", rel), join("charter-files", rel));
}
console.log(`charter-files/: ${charterFiles.length} files`);
