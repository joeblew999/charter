// npm pack (prepack): charter's files, which adopt writes into every repo, copied into charter-files/
// from charter's repo, where they are changed; and charter's mise.toml, whose pins a repo's starts from.
import { cpSync, mkdirSync, rmSync } from "node:fs";
import { dirname, join } from "node:path";
import { charterFiles } from "./src/charter-files.ts";

rmSync("charter-files", { recursive: true, force: true });
for (const rel of [...charterFiles, "mise.toml"]) {
	mkdirSync(dirname(join("charter-files", rel)), { recursive: true });
	cpSync(join("..", rel), join("charter-files", rel));
}
console.log(`charter-files/: ${charterFiles.length} files, and mise.toml`);
