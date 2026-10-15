// This repo's CI: the jobs, and when GitHub runs them. `npm run adopt` writes a workflow per job from
// this (.github/workflows/self-<job>.yml); each installs the tools (mise), the packages, and runs
// `npm run ci -- <job>`, which is the command here too. `npm run ci` is every job, charter's repo job
// among them, one after another; `npm run check` the jobs in check, which need no Docker and no Cloudflare.
import { defineCI } from "charter-ts/ci";

export default defineCI({
	check: ["packages", "repo", "projects"],
	jobs: {
		packages: {
			about: "charter-ts and charter-lib with their tests, the Go library, this repo's own TypeScript, the projects kept the same, and these workflows linted",
			on: ["push", "pull_request", "tag"],
			os: ["linux", "macos"],
			timeoutMinutes: 20,
			run: [
				["tsc"],
				// The projects, kept the same: their scripts and the versions they share.
				["node", "--test", "projects.test.ts"],
				["npm", "run", "check", "-w", "ts/lib"],
				["npm", "run", "check", "-w", "charter-ts"],
				["charter-ts", "check", "go/lib"],
				// The workflows, linted (rhysd/actionlint, which mise loads at this version).
				["mise", "exec", "actionlint@1.7.12", "--", "actionlint"],
			],
		},
		projects: {
			about: "Every project's check: lint, its specs, its tests natively (Go) and under workerd",
			on: ["push", "pull_request", "tag"],
			each: "project",
			run: [["charter-ts", "check"]],
		},
		// Each API that is here in TypeScript and in Go answers the same in both: the TypeScript project is
		// the reference, and every request its specs give rise to goes to both (charter-ts twins).
		twins: {
			about: "Each API's TypeScript and Go projects under workerd, the same requests to both, and no answer may differ",
			on: ["push", "pull_request", "tag"],
			timeoutMinutes: 30,
			run: [
				["charter-ts", "twins", "ts/conformance", "go/conformance"],
				["charter-ts", "twins", "ts/examples-notes", "go/examples-notes"],
				["charter-ts", "twins", "ts/examples-start", "go/examples-start"],
			],
		},
		// What a user gets: a project made by charter-ts new from the package as a release packs it,
		// installed, and its own check passing (each template).
		new: {
			about: "A project made by charter-ts new from the packed package, as a release ships it, installed, and its check passing",
			on: ["push", "pull_request", "tag"],
			timeoutMinutes: 20,
			run: [["node", "charter-ts/test/new-from-package.ts"]],
		},
		// The conformance projects carry every Fern feature, so their clients prove the generators; an
		// example's specs are checked by Fern in its own check, and its committed Go SDK by a hash.
		clients: {
			about: "Every client the conformance projects' fern/generators.yml declares, generated with Fern and proven in its language",
			on: ["push", "pull_request", "tag"],
			each: "project",
			projects: ["go/conformance", "ts/conformance"],
			timeoutMinutes: 60,
			run: [["charter-ts", "sdk", "--prove"]],
		},
		deploy: {
			about: "A project's Worker deployed to Cloudflare, then its live tests against it",
			on: ["dispatch"],
			each: "project",
			projects: ["go/conformance", "ts/conformance", "go/examples-notes", "go/examples-start", "go/examples-start-htmx", "go/examples-start-datastar", "ts/examples-notes", "ts/examples-start"],
			secrets: ["CLOUDFLARE_API_TOKEN", "CLOUDFLARE_ACCOUNT_ID", "READ_TOKEN", "WRITE_TOKEN"],
			timeoutMinutes: 30,
			run: [["charter-ts", "deploy"], ["charter-ts", "live-test"]],
		},
	},
});
