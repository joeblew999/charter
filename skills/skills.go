// Package skills holds the agent skills that ship with charter's Go side; go tool charter skill installs them into an
// app's .claude/skills.
package skills

import "embed"

// FS is every skill directory: charter/SKILL.md.
//
//go:embed charter
var FS embed.FS
