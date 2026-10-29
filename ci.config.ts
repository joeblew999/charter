// This repo's own CI: its jobs, and when GitHub runs them. `npm run adopt` writes a workflow per job
// from this (.github/workflows/self-<job>.yml), each calling charter's job runner, which runs the job as
// `npm run ci -- <job>` runs it here. `npm run ci` runs every job GitHub runs on a change (all but
// deploy), charter's repo job among them; `npm run check` the jobs in check, which need no Cloudflare
// account (Docker, for the conformance projects' SDKs).
import type { CI } from "charter-ts/ci";

export default {
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
				["npm", "run", "check", "-w", "ts"],
				["npm", "run", "check", "-w", "charter-ts"],
				["charter-ts", "check", "."],
				// The release's config for the charter binary (.goreleaser.yaml), checked.
				["goreleaser", "check"],
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
				["charter-ts", "twins", "examples/conformance-ts", "examples/conformance-go"],
				["charter-ts", "twins", "examples/notes-ts", "examples/notes-go"],
				["charter-ts", "twins", "examples/start-ts", "examples/start-go"],
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
		// What an outside Go app gets: one made by charter init on this charter, with no TinyGo of its own, gets the
		// TinyGo its patch is for from charter, every way an app builds, and builds; its own TinyGo line is refused.
		"go-app": {
			about: "A Go app made by charter init gets charter's TinyGo through mise, every way it builds, builds, and may not pin its own",
			on: ["push", "pull_request", "tag"],
			timeoutMinutes: 20,
			run: [["bash", "examples/new-app.sh"]],
		},
		// The conformance projects carry every Fern feature, so their clients prove the generators; an
		// example's specs are checked by Fern in its own check, and its committed Go SDK by a hash.
		clients: {
			about: "Every client the conformance projects' fern/generators.yml declares, generated with Fern and proven in its language",
			on: ["push", "pull_request", "tag"],
			each: "project",
			projects: ["examples/conformance-go", "examples/conformance-ts"],
			timeoutMinutes: 60,
			run: [["charter-ts", "sdk", "--prove"]],
		},
		deploy: {
			about: "A project's Worker deployed to Cloudflare, then its live tests against it",
			on: ["dispatch"],
			each: "project",
			secrets: ["CLOUDFLARE_API_TOKEN", "CLOUDFLARE_ACCOUNT_ID", "READ_TOKEN", "WRITE_TOKEN"],
			timeoutMinutes: 30,
			run: [["charter-ts", "deploy"], ["charter-ts", "live-test"]],
		},
	},
} satisfies CI;
