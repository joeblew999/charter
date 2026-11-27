package command

import (
	"fmt"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/internal/tools"
	"log"
	"os"
	"os/exec"
	"strings"
)

// Env runs charter env: what this machine has of what an app needs, and what is missing. mise is all it must have:
// it brings Go, Node and fnox at the versions the app pins; the rest is for one job each.
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
	show("mise", tools.NoMise, true, "mise", "--version")
	show("go", "mise install", true, "go", "version")
	show("node", "mise install", true, "node", "--version")
	show("npm", "it comes with Node", true, "npm", "--version")
	show("git", "charter create copies a template with it", false, "git", "--version")
	show("claude", "only charter run intl:translate needs it (Claude Code)", false, "claude", "--version")
	// fnox, as mise has it for this folder, and a container engine.
	fnox, containers := tools.State()
	fmt.Printf("%s fnox       %s: Cloudflare's credentials, from the keychain, for deploy\n", mark(fnox), fnox)
	fmt.Printf("%s containers %s: for charter pack, which generates the SDKs\n", mark(containers), containers)
	if p, err := project.Load("."); err == nil {
		// What runs, against what the app pins.
		if tools.Mise() != "" {
			if pins, err := effectivePins(p); err == nil {
				var same, file []string
				for _, name := range pinned {
					short := strings.TrimPrefix(name, "github:jdx/")
					want, ok := pins[name]
					switch got := running(p, name); {
					case !ok:
						missing = true
						fmt.Printf("✗ pins       mise.toml does not pin %s\n", short)
					case got == want.Version:
						same = append(same, short+" "+got)
						file = append(file, want.File)
					default:
						missing = true
						fmt.Printf("✗ pins       %s %s runs here; %s pins %s: mise install, then a new shell\n", short, orNone(got, "none"), want.File, want.Version)
					}
				}
				if len(same) > 0 {
					fmt.Printf("✓ pins       %s run here, as %s pins\n", strings.Join(same, ", "), file[0])
				}
			}
		}
		if state, ok := devState(p); ok {
			fmt.Printf("✓ dev        running at %s (the Worker itself: %s; its log: %s)\n", state.URL, state.Worker, state.Log)
		} else {
			fmt.Println("· dev        not running here: go tool charter dev")
		}
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
			if _, err := credentials(p); err == nil {
				fmt.Println("✓ cloudflare  the token and the account's id are in fnox")
			} else {
				fmt.Println("· cloudflare  " + strings.ReplaceAll(err.Error(), "\n", "\n              "))
			}
		}
	}
	if missing {
		os.Exit(1)
	}
}

// mark is ✓ for a tool that is there, · for one charter install has yet to bring.
func mark(state string) string {
	if strings.HasPrefix(state, "not yet") {
		return "·"
	}
	return "✓"
}
