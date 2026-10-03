package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func init() {
	commands["github-secrets"] = command{"<NAME>...",
		"copy environment variables into this repo's GitHub Actions secrets (values are never printed)", githubSecrets}
	anywhere["github-secrets"] = true
	commands["deploy"] = command{"[cf deploy flags]",
		"deploy the Worker with the secrets WORKER_SECRETS names, from the environment (values are never printed), then apply pending migrations", deploy}
}

// githubSecrets sets each named variable as a repository secret. The value goes to `gh secret set`
// on standard input, so it is in no command line and no output. Run it under whatever holds the
// values: mise run cloudflare:secrets wraps it in `fnox exec`.
func githubSecrets(names []string) error {
	names = append(names, workerSecretNames()...) // the deploy workflow passes them to mise run deploy
	if len(names) == 0 {
		return errors.New("github-secrets needs the names of the variables to copy")
	}
	for _, name := range names {
		value := os.Getenv(name)
		if value == "" {
			return fmt.Errorf("%s is not set here: run this under what holds it (mise run cloudflare:secrets uses fnox)", name)
		}
		cmd := exec.Command("gh", "secret", "set", name)
		cmd.Stdin, cmd.Stderr = strings.NewReader(value), os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		fmt.Printf("set the GitHub secret %s\n", name)
	}
	return nil
}

// The Worker's secrets: the names WORKER_SECRETS (in mise.toml) lists, e.g. READ_TOKEN WRITE_TOKEN.
func workerSecretNames() []string { return strings.Fields(os.Getenv("WORKER_SECRETS")) }

// secretsFile writes the secrets, by name, to a file only this user can read, for cf deploy
// --secrets-file. The caller removes it. "" when there are none.
func secretsFile(values map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	file, err := os.CreateTemp("", "charter-secrets-*.json") // mode 0600
	if err != nil {
		return "", err
	}
	if err := json.NewEncoder(file).Encode(values); err != nil {
		file.Close()
		os.Remove(file.Name())
		return "", err
	}
	return file.Name(), file.Close()
}

// deploy deploys the Worker with its secrets, then applies pending migrations. The secrets come
// from the environment (mise run deploy takes them from fnox, or from GitHub's secrets in CI) and go
// through a file, so they are in no command line and no output; a new Worker is created with them,
// and a changed token is set by the next deploy.
func deploy(args []string) error {
	values := map[string]string{}
	for _, name := range workerSecretNames() {
		if values[name] = os.Getenv(name); values[name] == "" {
			return fmt.Errorf("%s is not set here: the Worker needs it (WORKER_SECRETS); mise run deploy takes it from fnox, CI from the repo's GitHub secrets", name)
		}
	}
	file, err := secretsFile(values)
	if err != nil {
		return err
	}
	if file != "" {
		defer os.Remove(file)
		args = append(args, "--secrets-file", file)
	}
	if err := sh(".", npmBin("cf"), append([]string{"deploy"}, args...)...); err != nil {
		return err
	}
	return migrate(nil)
}
