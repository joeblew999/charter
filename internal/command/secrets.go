package command

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/tools"
)

// The deployed Worker's secrets. Their names are the app's: .dev.vars.example lists each with a throwaway value for
// wrangler dev. Their real values are fnox's to keep, declared in the app's own fnox.toml (https://fnox.jdx.dev),
// which is committed: a secret there has its plain name (ADMIN_TOKEN) and says which provider holds its value (the
// keychain, a password manager, an encrypted file), never the value. So each app has its own secrets, a team picks
// its own store, and nothing of an app's is in a developer's global config.
//
// An environment of wrangler.jsonc is a profile of fnox.toml: the same names, other values.
//
//	charter run secrets [env]                     which the app names, and which fnox has
//	charter run secrets:put [env] NAME --random   a new random value, kept by fnox and put on the deployed Worker
//	charter run secrets:put [env] NAME            the value fnox has (or one piped in), put on the deployed Worker
//	                                              (--provider <name>: where to keep it, when fnox has several)
//
// charter test --deployed (and --env) hands the tests each one fnox has, under its own name: a test asks for it with
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

// fnoxIn is fnox run in the app's folder, where its fnox.toml is, for an environment's profile ("" the default).
func fnoxIn(p *project.Project, fnox, env string, words ...string) *exec.Cmd {
	if env != "" {
		words = append([]string{"-P", env}, words...)
	}
	cmd := exec.Command(fnox, words...)
	cmd.Dir = p.Root
	return cmd
}

// stored is where a provider keeps a secret of this Worker: a name of its own in a store every app shares (a
// keychain has one list), so it carries charter's name, the Worker's and the environment's.
func stored(p *project.Project, env, name string) string {
	word := func(s string) string {
		return strings.ToUpper(strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
				return r
			}
			return '_'
		}, s))
	}
	key := "CHARTER_" + word(p.Worker.Name)
	if env != "" {
		key += "_" + word(env)
	}
	return key + "_" + name
}

// declared reports whether the app's own fnox files name the secret (for the environment's profile): one that only
// a developer's global config has is not the app's.
func declared(p *project.Project, env, name string) bool {
	for _, file := range []string{"fnox.toml", ".fnox.toml", "fnox." + env + ".toml"} {
		b, err := os.ReadFile(filepath.Join(p.Root, file))
		if err != nil {
			continue
		}
		table := ""
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "[") {
				table = strings.Trim(line, "[] ")
				continue
			}
			key, _, ok := strings.Cut(line, "=")
			if !ok || strings.TrimSpace(key) != name {
				continue
			}
			switch {
			case env == "" && table == "secrets", env != "" && table == "profiles."+env+".secrets", env != "" && table == "secrets" && file == "fnox."+env+".toml":
				return true
			}
		}
	}
	return false
}

// keptSecret is the value fnox has for one of the app's secrets. It is handed on, never shown.
func keptSecret(p *project.Project, fnox, env, name string) (string, bool) {
	if !declared(p, env, name) {
		return "", false
	}
	out, err := fnoxIn(p, fnox, env, "get", name).Output()
	value := strings.TrimSpace(string(out))
	return value, err == nil && value != ""
}

// before is a secret as charter kept it before the app had a fnox.toml: in the developer's global config, under the
// Worker's name. It is read still, and moved by charter run secrets:put.
func before(p *project.Project, fnox, name string) (value, key string, ok bool) {
	for _, key := range []string{stored(p, "", name), strings.TrimPrefix(stored(p, "", name), "CHARTER_")} {
		cmd := exec.Command(fnox, "get", key)
		cmd.Dir = os.TempDir() // the global config alone
		if out, err := cmd.Output(); err == nil && strings.TrimSpace(string(out)) != "" {
			return strings.TrimSpace(string(out)), key, true
		}
	}
	return "", "", false
}

// envOf is the environment of wrangler.jsonc that is deployed at base, "" for the Worker itself.
func envOf(p *project.Project, base string) string {
	for env, at := range p.URLs {
		if strings.TrimRight(at, "/") == strings.TrimRight(base, "/") {
			return env
		}
	}
	return ""
}

// deployedSecrets are the app's secrets that fnox has for an environment, by their own names: what a test of the
// deployed Worker is handed. One the environment has already (CI's) is left to it.
func deployedSecrets(p *project.Project, env string) map[string]string {
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
		if value, ok := keptSecret(p, fnox, env, name); ok {
			out[name] = value
			say("%s: from fnox (%s)", name, profile(env))
		} else if value, key, ok := before(p, fnox, name); ok && env == "" {
			out[name] = value
			say("%s: from fnox's global config (%s): go tool charter run secrets:put %s moves it into the app's fnox.toml", name, key, name)
		} else {
			say("· %s: not in fnox (go tool charter run secrets:put %s%s): a test that needs it is skipped", name, envWord(env), name)
		}
	}
	return out
}

func profile(env string) string {
	if env == "" {
		return "fnox.toml"
	}
	return "fnox.toml, profile " + env
}

func envWord(env string) string {
	if env == "" {
		return ""
	}
	return env + " "
}

// takeEnv takes a leading environment name (one of wrangler.jsonc's env) off args.
func takeEnv(p *project.Project, args *[]string) string {
	if flags := envArgs(p, args); len(flags) == 2 {
		return flags[1]
	}
	return ""
}

func secrets(p *project.Project, args []string) {
	env := takeEnv(p, &args)
	if len(args) > 0 {
		check(errors.New("usage: charter run secrets [env]   (charter run secrets:put [env] NAME [--random] puts one)"))
	}
	names := secretNames(p)
	if len(names) == 0 {
		say("the app names no secrets: %s.example lists them, each with a throwaway value for wrangler dev", project.DevVars)
		return
	}
	fnox, err := tools.Fnox()
	check(err)
	for _, name := range names {
		mark, what := "·", "not in fnox: go tool charter run secrets:put "+envWord(env)+name
		if _, ok := keptSecret(p, fnox, env, name); ok {
			mark, what = "✓", "in fnox ("+profile(env)+")"
		} else if _, key, ok := before(p, fnox, name); ok && env == "" {
			what = "in fnox's global config as " + key + ": go tool charter run secrets:put " + name + " moves it into the app's fnox.toml"
		}
		say("%s %-16s %s", mark, name, what)
	}
}

// provider is the fnox provider a new secret of the app is kept with: the one asked for, else the one the app's
// secrets use already, else the one that keeps Cloudflare's token. Never none: fnox would write the value into
// fnox.toml as it is, and that file is committed.
func provider(p *project.Project, fnox, asked string) (string, error) {
	out, _ := fnoxIn(p, fnox, "", "list").Output()
	of := map[string]string{} // key → provider
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[1] == "provider" {
			of[fields[0]] = strings.Trim(fields[2], "()")
		}
	}
	if asked != "" {
		return asked, nil
	}
	for _, name := range secretNames(p) {
		if declared(p, "", name) && of[name] != "" {
			return of[name], nil
		}
	}
	if of["CLOUDFLARE_API_TOKEN"] != "" {
		return of["CLOUDFLARE_API_TOKEN"], nil
	}
	return "", errors.New("fnox has no provider to keep a secret with, and without one it would write the value into fnox.toml, which is committed. Add one (" +
		fnox + " provider add; the keychain, a password manager, or age for an encrypted file), or name it: charter run secrets:put NAME --provider <name>")
}

func secretsPut(p *project.Project, args []string) {
	env := takeEnv(p, &args)
	random, with := false, ""
	args = slices.Clone(args)
	if i := slices.Index(args, "--random"); i >= 0 {
		args, random = slices.Delete(args, i, i+1), true
	}
	if i := slices.Index(args, "--provider"); i >= 0 && i+1 < len(args) {
		with = args[i+1]
		args = slices.Delete(args, i, i+2)
	}
	if len(args) != 1 {
		check(errors.New("usage: charter run secrets:put [env] NAME [--random] [--provider <name>]"))
	}
	name := args[0]
	if !slices.Contains(secretNames(p), name) {
		check(fmt.Errorf("%s is not a secret the app names: add it to %s.example first, with a throwaway value for wrangler dev", name, project.DevVars))
	}
	fnox, err := credentials(p)
	check(err)
	value, have := keptSecret(p, fnox, env, name)
	// One kept in the developer's global config, as charter kept them before, moves into the app's fnox.toml.
	was := ""
	if !have && !random && env == "" {
		value, was, _ = before(p, fnox, name)
	}
	piped := func() bool {
		info, err := os.Stdin.Stat()
		return err == nil && info.Mode()&os.ModeCharDevice == 0
	}()
	if piped && !random {
		b, err := io.ReadAll(os.Stdin)
		check(err)
		if value = strings.TrimSpace(string(b)); value == "" {
			check(errors.New("nothing was piped in"))
		}
	}
	if !random && value == "" {
		check(fmt.Errorf("fnox has no %s for %s. Either:\n  go tool charter run secrets:put %s%s --random     a new random value (a token of the app's own)\n  echo … | go tool charter run secrets:put %s%s      a value piped in", name, profile(env), envWord(env), name, envWord(env), name))
	}
	if random || !have || piped {
		keep, err := provider(p, fnox, with)
		check(err)
		// Declared in the app's fnox.toml, the value with the provider under a name of its own. A developer who
		// has the file and not the value (a new checkout) is told by charter, not stopped by fnox.
		words := []string{"set", "--provider", keep, "--key-name", stored(p, env, name), "--if-missing", "ignore"}
		if random {
			words = append(words, "--generate", "--generate-type", "hex", "--length", "64", "--force")
		}
		set := fnoxIn(p, fnox, env, append(words, name)...)
		set.Stderr = os.Stderr
		if !random {
			// Through its stdin: a value on a command line is in the process list.
			set.Stdin = strings.NewReader(value)
		}
		check(set.Run())
		if value, have = keptSecret(p, fnox, env, name); !have {
			check(fmt.Errorf("fnox took %s and does not give it back: %s doctor", name, fnox))
		}
		say("✓ %s is fnox's (%s, kept by %s as %s)", name, profile(env), keep, stored(p, env, name))
	}
	if was != "" {
		rm := exec.Command(fnox, "remove", "--global", was)
		rm.Dir = os.TempDir()
		if rm.Run() == nil {
			say("✓ its old key %s is gone from fnox's global config", was)
		}
	}
	check(putSecret(p, fnox, env, name, value))
	say("✓ %s is on the deployed Worker %s%s", name, p.Worker.Name, map[bool]string{true: " (" + env + ")"}[env != ""])
}

// putSecret puts a value on the deployed Worker as its secret, through wrangler's stdin.
func putSecret(p *project.Project, fnox, env, name, value string) error {
	bin, err := npm(p, "wrangler")
	if err != nil {
		return err
	}
	words := []string{"secret", "put", name}
	if env != "" {
		words = append(words, "--env", env)
	}
	put := exec.Command(fnox, append([]string{"exec", "--", bin}, words...)...)
	if os.Getenv("CLOUDFLARE_API_TOKEN") != "" {
		put = exec.Command(bin, words...)
	}
	var said strings.Builder
	put.Dir, put.Stdin, put.Stdout, put.Stderr = p.Root, strings.NewReader(value), &said, &said
	if err := put.Run(); err != nil {
		return fmt.Errorf("wrangler secret put %s: %w\n%s", name, err, strings.TrimSpace(said.String()))
	}
	return nil
}

// onWorker are the names of the secrets the deployed Worker has, ok false when there is no such Worker.
func onWorker(p *project.Project, env string) (names []string, ok bool) {
	words := []string{"secret", "list"}
	if env != "" {
		words = append(words, "--env", env)
	}
	out, err := cloudflareOutput(p, words...)
	if err != nil {
		return nil, false
	}
	for _, m := range secretListed.FindAllStringSubmatch(out, -1) {
		names = append(names, m[1])
	}
	return names, true
}

var secretListed = regexp.MustCompile(`"name":\s*"([^"]+)"`)

// pushSecrets puts on the deployed Worker each of the app's secrets that fnox has and the Worker lacks: what a
// Worker deployed for the first time, or again after it was deleted, needs before it is asked anything.
func pushSecrets(p *project.Project, env string) {
	if os.Getenv("CLOUDFLARE_API_TOKEN") != "" || len(secretNames(p)) == 0 {
		return // CI: its secrets are the pipeline's to put
	}
	fnox, err := tools.Fnox()
	if err != nil {
		return
	}
	has, ok := onWorker(p, env)
	if !ok {
		return
	}
	for _, name := range secretNames(p) {
		if slices.Contains(has, name) {
			continue
		}
		value, kept := keptSecret(p, fnox, env, name)
		if !kept {
			say("· the Worker has no %s, and neither has fnox: go tool charter run secrets:put %s%s", name, envWord(env), name)
			continue
		}
		check(putSecret(p, fnox, env, name, value))
		say("✓ %s is on the deployed Worker (from fnox)", name)
	}
}
