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

test("a new Go project's mise.toml carries charter's pins for every tool its build runs", async () => {
	const { miseStarter } = await import("./charter-files.ts");
	const started = miseStarter(true, true);
	const charters = readFileSync(charterMise(), "utf8");
	for (const tool of ["node", "github:jdx/fnox", "go", "tinygo", "aqua:WebAssembly/binaryen", "aqua:sqlc-dev/sqlc"]) {
		assert.equal(pinIn(started, tool), pinIn(charters, tool), `${tool} in a new Go project's mise.toml`);
	}
	assert.equal(pinIn(miseStarter(false, false), "tinygo"), undefined, "a repo without Go gets no TinyGo");
});

test("charter's mise.toml pins every tool charter-ts runs through mise", () => {
	const text = readFileSync(charterMise(), "utf8");
	for (const tool of ["go", "node", "tinygo", "aqua:WebAssembly/binaryen", "aqua:sqlc-dev/sqlc", "github:jdx/fnox"]) {
		assert.ok(pinIn(text, tool), `${tool} is pinned in charter's mise.toml`);
	}
});
