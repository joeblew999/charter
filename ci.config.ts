// This repo's CI: the jobs, and when GitHub runs them. `npx charter-ts ci:write` writes a workflow per
// job from this (.github/workflows/self-<job>.yml); each installs the tools (mise), the packages, and runs
// `npx charter-ts ci <job>`, which is the command here too. `npm run ci` is every job, one after
// another; `npm run check` the two that need no Docker.
import { defineCI } from "charter-ts/ci";

export default defineCI({
	jobs: {
		check: {
			about: "charter-ts and charter-lib with their tests, the Go library, this repo's own TypeScript, the projects kept the same, the docs, and these workflows (written from here, and linted)",
			on: ["push", "pull_request", "tag"],
			os: ["linux", "macos"],
			timeoutMinutes: 20,
			run: [
				["tsc"],
				// The projects, kept the same: their scripts and the versions they share.
				["node", "--test", "projects.test.ts"],
				["npm", "run", "check", "-w", "ts/lib"],
				["npm", "run", "check", "-w", "charter-ts"],
				["charter-ts", "go:check", "go/lib"],
				// The kit, the workflows (written from this file) and the docs.
				["charter-ts", "docs:check"],
				// The workflows, linted (rhysd/actionlint, which mise loads at this version).
				["mise", "exec", "actionlint@1.7.12", "--", "actionlint"],
			],
		},
		// What any repo charter keeps runs (the kit starts it in one with no API): the repo on GitHub as
		// package.json says, and the upstream issues the code works around. It reads GitHub, so it waits
		// for no Docker and no Cloudflare.
		repo: {
			about: "What charter checks in any repo: the kit and the docs (docs:check), the repo on GitHub as package.json says (repo --check), the upstream issues the code works around",
			on: ["push", "pull_request"],
			run: [["charter-ts", "repo:ci"]],
			timeoutMinutes: 10,
			// What the checks read with gh, and nothing they could change.
			permissions: { contents: "read", issues: "read", "pull-requests": "read", pages: "read" },
		},
		projects: {
			about: "Every project's check: lint, its specs, its tests natively (Go) and under workerd",
			on: ["push", "pull_request", "tag"],
			each: "project",
			run: [["charter-ts", "check"]],
		},
		// The conformance projects carry every Fern feature, so their clients prove the generators; an
		// example's specs are checked by Fern in its own check, and its committed Go SDK by a hash.
		clients: {
			about: "Every client the conformance projects' fern/generators.yml declares, generated with Fern and proven in its language",
			on: ["push", "pull_request", "tag"],
			each: "project",
			projects: ["go/conformance", "ts/conformance"],
			timeoutMinutes: 60,
			run: [["charter-ts", "clients"]],
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
