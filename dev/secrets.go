package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func init() {
	commands["github-secrets"] = command{"<NAME>...",
		"copy environment variables into this repo's GitHub Actions secrets (values are never printed)", githubSecrets}
}

// githubSecrets sets each named variable as a repository secret. The value goes to `gh secret set`
// on standard input, so it is in no command line and no output. Run it under whatever holds the
// values: mise run cloudflare:secrets wraps it in `fnox exec`.
func githubSecrets(names []string) error {
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
