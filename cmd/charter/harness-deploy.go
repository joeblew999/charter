package main

// charter harness-deploy: REMOTE: deploy the project's Worker twice: as itself and, --mode api, as <worker>-api
// Run by: mise run deploy (the examples' mise.toml)
func init() {
	commands["harness-deploy"] = command{"", "REMOTE: deploy the project's Worker twice: as itself and, --mode api, as <worker>-api", harnessDeploy}
}

func harnessDeploy([]string) error {
	if err := harnessSync(nil); err != nil {
		return err
	}
	for _, mode := range []string{"production", "api"} {
		if err := sh(".", npmBin("cf"), "deploy", "--mode", mode); err != nil {
			return err
		}
	}
	return nil
}
