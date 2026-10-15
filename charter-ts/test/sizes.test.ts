// What charter-ts says of what it makes (scripts.ts: made): a file's size and a folder's (the sum of
// its files), gzipped, its limit, and how it changed since it was last made there.
//   node --test test/*.test.ts   (npm run check -w charter-ts runs them)
import assert from "node:assert/strict";
import { randomBytes } from "node:crypto";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, it } from "node:test";
import { made, madeThisRun } from "../src/scripts.ts";

const scratch = mkdtempSync(join(tmpdir(), "charter-ts-sizes-"));
after(() => rmSync(scratch, { recursive: true, force: true }));

/** What made printed, and what it returned. */
function said(path: string, what: string, limit?: number): { line: string; gzipped: number } {
	const lines: string[] = [];
	const log = console.log;
	console.log = (line: string) => lines.push(line);
	try {
		return { gzipped: made(path, what, limit), line: lines.join("\n") };
	} finally {
		console.log = log;
	}
}

it("says a file's size, gzipped and against its limit, then how it changed since", () => {
	const file = join(scratch, "app.wasm");
	writeFileSync(file, "x".repeat(2_000_000));
	const first = said(file, "the Worker's Wasm", 3_000_000);
	assert.match(first.line, /^made .*app\.wasm \(the Worker's Wasm\): 2\.00 MB, \d+\.\d kB gzipped \(limit 3\.00 MB\)$/);
	assert.match(said(file, "the Worker's Wasm", 3_000_000).line, /\(limit 3\.00 MB; the same as last time\)$/);
	// Random bytes do not compress: 300 kB more, gzipped.
	writeFileSync(file, randomBytes(300_000));
	assert.match(said(file, "the Worker's Wasm").line, /: 0\.30 MB, 0\.30 MB gzipped \(\+0\.30 MB gzipped since last time\)$/);
	assert.deepEqual(madeThisRun.at(-1), { what: "the Worker's Wasm", gzipped: madeThisRun.at(-1)!.gzipped, limit: undefined });
});

it("says a folder's size as the sum of its files", () => {
	const folder = join(scratch, "bundle");
	mkdirSync(join(folder, "assets"), { recursive: true });
	writeFileSync(join(folder, "index.js"), "a".repeat(500_000));
	writeFileSync(join(folder, "assets", "app.wasm"), "b".repeat(1_500_000));
	assert.match(said(folder, "the Worker's bundle").line, /\(the Worker's bundle\): 2\.00 MB, /);
});
