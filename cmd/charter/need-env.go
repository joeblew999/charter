package main

import (
	"fmt"
	"os"
	"strings"
)

// charter need-env: fail, naming them, unless these environment variables are set (the secrets a workflow needs)
// Run by: mise run cloudflare:token (tasks/repo/tasks.toml)
func init() {
	commands["need-env"] = command{"<NAME>...", "fail, naming them, unless these environment variables are set (the secrets a workflow needs)", needEnv}
	anywhere["need-env"] = true
}

func needEnv(names []string) error {
	var missing []string
	for _, name := range names {
		if os.Getenv(name) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("not set: %s. On GitHub, add each as a repository secret (gh secret set <NAME>, or Settings > Secrets and variables > Actions); locally, export them",
			strings.Join(missing, ", "))
	}
	return nil
}
