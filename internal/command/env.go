package command

import (
	"fmt"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"log"
	"os"
	"os/exec"
	"strings"
)

// Env runs charter env: what this machine has of what an app needs, and what is missing. Go and Node are all it must
// have; the rest is for one job each.
func Env(args []string) {
	log.SetFlags(0)
	missing := false
	show := func(name, why string, required bool, cmd string, a ...string) {
		out, err := exec.Command(cmd, a...).Output()
		line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
		switch {
		case err == nil:
			fmt.Printf("✓ %-10s %s\n", name, line)
		case required:
			missing = true
			fmt.Printf("✗ %-10s not found: %s\n", name, why)
		default:
			fmt.Printf("· %-10s not found: %s\n", name, why)
		}
	}
	show("go", "https://go.dev/dl/", true, "go", "version")
	show("node", "https://nodejs.org", true, "node", "--version")
	show("npm", "it comes with Node", true, "npm", "--version")
	show("git", "charter create copies a template with it", false, "git", "--version")
	show("docker", "only charter pack needs it (Fern generates SDKs in containers)", false, "docker", "--version")
	show("claude", "only charter run intl:translate needs it (Claude Code)", false, "claude", "--version")
	show("fnox", "optional: Cloudflare credentials from the keychain for charter deploy", false, "fnox", "--version")
	if p, err := project.Load("."); err == nil {
		for _, tool := range []string{"wrangler", "vite"} {
			if tool == "vite" && !hasVite(p) {
				continue
			}
			if bin := run.Bin(tool, p.Root); bin != tool {
				show(tool, "", true, bin, "--version")
			} else {
				missing = true
				fmt.Printf("✗ %-10s not installed here: charter install\n", tool)
			}
		}
		if os.Getenv("CLOUDFLARE_API_TOKEN") != "" {
			fmt.Println("✓ cloudflare  CLOUDFLARE_API_TOKEN is set")
		} else {
			fmt.Println("· cloudflare  no CLOUDFLARE_API_TOKEN in the environment: charter deploy uses fnox when it is installed, else wrangler login")
		}
	}
	if missing {
		os.Exit(1)
	}
}
