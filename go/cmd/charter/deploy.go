package main

import (
	"fmt"
	"os"
)

// charter deploy: deploy the Worker with the secrets WORKER_SECRETS names, and those of WORKER_OPTIONAL_SECRETS that are set, from the environment (values are never printed), then apply pending migrations
// Run by: mise run deploy (the examples' mise.toml)
func init() {
	commands["deploy"] = command{"[cf deploy flags]",
		"deploy the Worker with the secrets WORKER_SECRETS names, and those of WORKER_OPTIONAL_SECRETS that are set, from the environment (values are never printed), then apply pending migrations", deploy}
}

// deploy deploys the Worker with its secrets, then applies pending migrations. The secrets come
// from the environment (mise run deploy takes them from fnox, or from GitHub's secrets in CI) and go
// through a file, so they are in no command line and no output; a new Worker is created with them,
// and a changed token is set by the next deploy. An optional secret goes with them when it is set
// (the project's cloudflare.config.ts declares it then).
func deploy(args []string) error {
	values := map[string]string{}
	for _, name := range workerSecretNames() {
		if values[name] = os.Getenv(name); values[name] == "" {
			return fmt.Errorf("%s is not set here: the Worker needs it (WORKER_SECRETS); mise run deploy takes it from fnox, CI from the repo's GitHub secrets", name)
		}
	}
	for _, name := range optionalSecretNames() {
		if value := os.Getenv(name); value != "" {
			values[name] = value
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
