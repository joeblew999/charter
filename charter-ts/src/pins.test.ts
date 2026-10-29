import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { charterMise, pinIn, pinned } from "./pins.ts";

test("pinIn reads bare, quoted and table pins, only under [tools]", () => {
	const text = [
		"[settings]",
		'tinygo = "9.9.9"',
		"[tools]",
		'go = "1.27.3"',
		'"aqua:sqlc-dev/sqlc" = "1.31.1"',
		'"github:jdx/fnox" = { version = "1.44.0", os = ["linux"] }',
		"# tinygo = \"0.0.1\"",
		'tinygo = "0.44.0" # the build patches it',
	].join("\n");
	assert.equal(pinIn(text, "go"), "1.27.3");
	assert.equal(pinIn(text, "aqua:sqlc-dev/sqlc"), "1.31.1");
	assert.equal(pinIn(text, "github:jdx/fnox"), "1.44.0");
	assert.equal(pinIn(text, "tinygo"), "0.44.0");
	assert.equal(pinIn(text, "rust"), undefined);
});

test("pinned: a repo's own mise.toml wins, charter's is the fallback, nothing pinned throws", () => {
	const repo = mkdtempSync(join(tmpdir(), "pins-"));
	const project = join(repo, "go", "app");
	mkdirSync(project, { recursive: true });
	writeFileSync(join(repo, "mise.toml"), '[tools]\ntinygo = "0.40.0"\n');
	assert.equal(pinned("tinygo", project), "0.40.0");
	const charters = pinIn(readFileSync(charterMise(), "utf8"), "aqua:WebAssembly/binaryen");
	assert.ok(charters, "charter's mise.toml pins binaryen");
	assert.equal(pinned("aqua:WebAssembly/binaryen", project), charters);
	assert.throws(() => pinned("no-such-tool", project), /no version in mise\.toml/);
});

test("a new Go project's mise.toml carries charter's pins, but not TinyGo's or wasm-opt's", async () => {
	const { miseStarter } = await import("./charter-files.ts");
	const started = miseStarter(true, true);
	const charters = readFileSync(charterMise(), "utf8");
	for (const tool of ["node", "github:jdx/fnox", "go", "aqua:sqlc-dev/sqlc"]) {
		assert.equal(pinIn(started, tool), pinIn(charters, tool), `${tool} in a new Go project's mise.toml`);
	}
	for (const tool of ["tinygo", "aqua:WebAssembly/binaryen"]) {
		assert.equal(pinIn(started, tool), undefined, `${tool}: charter pins it beside its patch (.config/mise/conf.d/charter.toml), not the repo`);
	}
});

test("charter's mise.toml pins every tool charter-ts runs through mise", () => {
	const text = readFileSync(charterMise(), "utf8");
	for (const tool of ["go", "node", "tinygo", "aqua:WebAssembly/binaryen", "aqua:sqlc-dev/sqlc", "github:jdx/fnox"]) {
		assert.ok(pinIn(text, tool), `${tool} is pinned in charter's mise.toml`);
	}
});

test("a Go repo gets the same TinyGo file go tool charter tasks writes, and its own TinyGo line is refused", async () => {
	const { goPinsConf, goPinsFile, ownToolPins } = await import("./charter-files.ts");
	const { charterRepo } = await import("./package.ts");
	assert.ok(charterRepo, "run from charter's repo");
	const goPins = readFileSync(join(charterRepo!, goPinsFile), "utf8");
	assert.equal(pinIn(goPins, "tinygo"), pinIn(readFileSync(charterMise(), "utf8"), "tinygo"), "the Go library's pins equal charter's mise.toml");
	const repo = mkdtempSync(join(tmpdir(), "go-repo-"));
	mkdirSync(join(repo, ".config", "mise", "conf.d"), { recursive: true });
	writeFileSync(join(repo, goPinsConf), goPins);
	writeFileSync(join(repo, "mise.toml"), '[tools]\ngo = "1.27.3"\n');
	writeFileSync(join(repo, "go.mod"), "module example.com/app\n");
	assert.deepEqual(ownToolPins(charterRepo!), [], "charter's own mise.toml says the version: not refused");
	assert.equal(pinned("tinygo", repo), pinIn(goPins, "tinygo"), "the build's TinyGo is the one charter's file pins");
	assert.deepEqual(ownToolPins(repo), []);
	writeFileSync(join(repo, "mise.toml"), '[tools]\ngo = "1.27.3"\ntinygo = "0.43.0"\n"github:WebAssembly/binaryen" = "version_133"\n');
	const refused = ownToolPins(repo);
	assert.equal(refused.length, 2, refused.join("\n"));
	assert.match(refused[0]!, /^mise\.toml pins tinygo: delete that line/);
	assert.match(refused[1]!, /^mise\.toml pins github:WebAssembly\/binaryen: delete that line/);
});
