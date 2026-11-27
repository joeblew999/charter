package command

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/tools"
)

// The deployed Worker's secrets. Their names are the app's: .dev.vars.example lists each with a throwaway value for
// wrangler dev. Their real values are kept where Cloudflare's credentials are, in fnox (the keychain, never a
// file), each under charter's name, the Worker's and its own: ADMIN_TOKEN of the Worker my-app is
// CHARTER_MY_APP_ADMIN_TOKEN. So one machine keeps the secrets of every app apart, and none of them can be taken
// for a key some other tool keeps there.
//
//	charter run secrets                    which the app names, and which fnox has
//	charter run secrets:put NAME --random  a new random value, kept in fnox and put on the deployed Worker
//	charter run secrets:put NAME           the value fnox has (or one piped in), put on the deployed Worker
//
// charter test --deployed hands the tests each one fnox has, under its own name: a test asks for it with
// apptest.Secret, as it does locally.

// secretNames are the secrets the app names, in the order .dev.vars.example has them.
func secretNames(p *project.Project) []string {
	b, err := os.ReadFile(filepath.Join(p.Root, project.DevVars+".example"))
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(string(b), "\n") {
		if name, _, ok := strings.Cut(strings.TrimSpace(line), "="); ok && !strings.HasPrefix(name, "#") {
			names = append(names, strings.TrimSpace(name))
		}
	}
	return names
}

// secretKey is where fnox keeps a secret of this Worker.
func secretKey(p *project.Project, name string) string { return "CHARTER_" + workerKey(p, name) }

// workerKey is where charter kept one before its keys began with its own name: read still, and moved by
// charter run secrets:put.
func workerKey(p *project.Project, name string) string {
	worker := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, p.Worker.Name)
	return strings.ToUpper(worker) + "_" + name
}

// keptSecret is the value fnox has under key. It is handed on, never shown.
func keptSecret(fnox, key string) (string, bool) {
	out, err := exec.Command(fnox, "get", key).Output()
	value := strings.TrimSpace(string(out))
	return value, err == nil && value != ""
}

// deployedSecrets are the app's secrets that fnox has, by their own names: what a test of the deployed Worker is
// handed. One the environment has already (CI's) is left to it.
func deployedSecrets(p *project.Project) map[string]string {
	names := secretNames(p)
	if len(names) == 0 {
		return nil
	}
	fnox, err := tools.Fnox()
	if err != nil {
		return nil
	}
	out := map[string]string{}
	for _, name := range names {
		if os.Getenv(name) != "" {
			continue
		}
		if value, ok := keptSecret(fnox, secretKey(p, name)); ok {
			out[name] = value
			say("%s: from fnox (%s)", name, secretKey(p, name))
		} else if value, ok := keptSecret(fnox, workerKey(p, name)); ok {
			out[name] = value
			say("%s: from fnox, under its old key (%s): go tool charter run secrets:put %s moves it to %s", name, workerKey(p, name), name, secretKey(p, name))
		} else {
			say("· %s: not in fnox (go tool charter run secrets:put %s): a test that needs it is skipped", name, name)
		}
	}
	return out
}

func secrets(p *project.Project, args []string) {
	if len(args) > 0 {
		check(errors.New("usage: charter run secrets   (charter run secrets:put NAME [--random] puts one)"))
	}
	names := secretNames(p)
	if len(names) == 0 {
		say("the app names no secrets: %s.example lists them, each with a throwaway value for wrangler dev", project.DevVars)
		return
	}
	fnox, err := tools.Fnox()
	check(err)
	for _, name := range names {
		key, mark, what := secretKey(p, name), "·", "not in fnox: go tool charter run secrets:put "+name
		if _, ok := keptSecret(fnox, key); ok {
			mark, what = "✓", "in fnox"
		} else if _, ok := keptSecret(fnox, workerKey(p, name)); ok {
			what = "in fnox under its old key " + workerKey(p, name) + ": go tool charter run secrets:put " + name + " moves it"
		}
		say("%s %-16s %s (%s)", mark, name, what, key)
	}
}

func secretsPut(p *project.Project, args []string) {
	random := false
	if i := slices.Index(args, "--random"); i >= 0 {
		args, random = slices.Delete(slices.Clone(args), i, i+1), true
	}
	if len(args) != 1 {
		check(errors.New("usage: charter run secrets:put NAME [--random]"))
	}
	name := args[0]
	if !slices.Contains(secretNames(p), name) {
		check(fmt.Errorf("%s is not a secret the app names: add it to %s.example first, with a throwaway value for wrangler dev", name, project.DevVars))
	}
	fnox, err := credentials(p)
	check(err)
	key := secretKey(p, name)
	value, have := keptSecret(fnox, key)
	// One kept under the key charter used before moves to the one it uses now.
	moved := false
	if old := workerKey(p, name); !have && !random {
		if was, ok := keptSecret(fnox, old); ok {
			value, moved = was, true
		}
	}
	piped := func() bool {
		info, err := os.Stdin.Stat()
		return err == nil && info.Mode()&os.ModeCharDevice == 0
	}
	switch {
	case random:
		b := make([]byte, 32)
		_, err := rand.Read(b)
		check(err)
		value = hex.EncodeToString(b)
	case piped():
		b, err := io.ReadAll(os.Stdin)
		check(err)
		if value = strings.TrimSpace(string(b)); value == "" {
			check(errors.New("nothing was piped in"))
		}
	case !have && !moved:
		check(fmt.Errorf("fnox has no %s. Either:\n  go tool charter run secrets:put %s --random     a new random value (a token of the app's own)\n  %s set --global %s     it asks for the value; then this again", key, name, fnox, key))
	}
	if random || !have || piped() {
		// Through its stdin: a value on a command line is in the process list.
		set := exec.Command(fnox, "set", "--global", key)
		set.Stdin, set.Stderr = strings.NewReader(value), os.Stderr
		check(set.Run())
		say("✓ %s is in fnox (%s)", name, key)
	}
	// The old key goes, whatever put the new value there: one secret, one place.
	if old := workerKey(p, name); exec.Command(fnox, "get", old).Run() == nil {
		if exec.Command(fnox, "remove", "--global", old).Run() == nil {
			say("✓ its old key %s is gone from fnox", old)
		}
	}
	bin, err := npm(p, "wrangler")
	check(err)
	put := exec.Command(fnox, "exec", "--", bin, "secret", "put", name)
	if os.Getenv("CLOUDFLARE_API_TOKEN") != "" {
		put = exec.Command(bin, "secret", "put", name)
	}
	put.Dir, put.Stdin, put.Stdout, put.Stderr = p.Root, strings.NewReader(value), os.Stdout, os.Stderr
	check(put.Run())
	say("✓ %s is on the deployed Worker %s", name, p.Worker.Name)
}
