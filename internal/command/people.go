package command

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/joeblew999/charter/api"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
)

// The app's own people (the sign-in screen, api.People), from the command line: who there is, and an invitation
// that makes whoever takes it up what it says. That is how an app gets its first admin, on a developer's machine
// and deployed: nobody is an admin until someone is invited to be. After that, an admin says who is what on the
// people page.
//
//	charter run people                          who signed up, and what each is
//	charter run people:invite admin             a link: whoever signs up through it, once, within the hour, is an admin
//	charter run people:invite admin --deployed  the same on the deployed Worker

// asked runs one statement on the app's database, the local one or with --remote the deployed one, and reads its
// rows into into (nil: none are wanted).
func asked(p *project.Project, binding, flag, sql string, into any) error {
	if flag == "--remote" {
		return remote(p, binding, sql, into)
	}
	bin, err := npm(p, "wrangler")
	if err != nil {
		return err
	}
	out, err := run.Output(p.Root, map[string]string{"WRANGLER_SEND_METRICS": "false", "CI": "1"}, bin, "d1", "execute", binding, "--local", "--json", "--command", sql)
	if err != nil {
		return fmt.Errorf("%w\n%s", err, strings.TrimSpace(out))
	}
	return results(out, into)
}

func people(p *project.Project, args []string) {
	binding, flag := where(p, &args)
	if len(args) > 0 {
		check(errors.New("usage: charter run people [--deployed]"))
	}
	var everyone []struct{ Name, Role, Created_at string }
	check(asked(p, binding, flag, "SELECT name, role, created_at FROM users ORDER BY created_at, id", &everyone))
	if len(everyone) == 0 {
		say("nobody has signed up. The first admin: go tool charter run people:invite admin%s", map[bool]string{true: " --deployed"}[flag == "--remote"])
		return
	}
	for _, person := range everyone {
		say("%-12s %-24s since %s", person.Role, person.Name, person.Created_at)
	}
}

var roleName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,39}$`)

func peopleInvite(p *project.Project, args []string) {
	binding, flag := where(p, &args)
	if len(args) != 1 || !roleName.MatchString(args[0]) {
		check(errors.New("usage: charter run people:invite <role> [--deployed]   (a role of the app's api.People: admin)"))
	}
	role := args[0]
	base := ""
	if flag == "--remote" {
		if base = strings.TrimRight(p.URL, "/"); base == "" {
			check(errors.New(`charter.toml has no url, where the Worker is deployed: url = "https://…"`))
		}
	} else if state, running := devState(p); running {
		base = strings.TrimRight(state.URL, "/")
	} else {
		check(errors.New("charter dev is not running here, and an invitation is taken up on its sign-in page: go tool charter dev, then this again"))
	}
	b := make([]byte, 24)
	_, err := rand.Read(b)
	check(err)
	code := "ch_" + hex.EncodeToString(b)
	// Kept as the app keeps one (api.Auth's Invite): by the hash of its code, for an hour.
	check(asked(p, binding, flag, fmt.Sprintf("INSERT INTO ceremonies (id, data, expires_at) VALUES ('%s', '%s', datetime('now', '+%d minutes'))",
		api.InviteID(code), role, int(api.InviteLasts.Minutes())), nil))
	say("An invitation to be %s on %s. It works once, within the hour: open it, give a name, and the device makes the passkey.", role, base)
	say("(A role the app does not have is refused there.)\n")
	say("%s%s?code=%s", base, app.InvitePath, code)
}
