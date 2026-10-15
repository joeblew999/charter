// Started by `charter-ts adopt` (github.com/joeblew999/charter): yours to edit.
// This repo's CI: its jobs, and when GitHub runs them, typed. `npx charter-ts ci:write` writes a
// workflow per job from it (.github/workflows/self-<job>.yml), each calling charter's job runner
// (charter-ci.yml); `npx charter-ts ci` runs every job here, the same way. Add a job of your own beside
// charter's: { about, on, run: [[program, ...args]] }.
import type { CI } from "charter-ts/ci";

export default {
	jobs: {
		repo: {
			about: "What charter checks in any repo: the kit and the docs (docs:check), the repo on GitHub as package.json says (repo --check), the upstream issues the code works around",
			on: ["push", "pull_request"],
			run: [["charter-ts", "repo:ci"]],
			timeoutMinutes: 10,
			// What the checks read with gh, and nothing they could change.
			permissions: { contents: "read", issues: "read", "pull-requests": "read", pages: "read" },
		},
	},
} satisfies CI;
