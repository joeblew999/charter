package main

// charter access:token: REMOTE
// Run by: mise run access:token (tasks/project/tasks.toml)
func init() {
	commands["access:token"] = command{"create <machine> <file|fnox> | list | revoke <machine>", "REMOTE: a machine's own service token for the Worker behind Access: create, list or revoke it. Secrets are never printed", func(args []string) error { return access(append([]string{"token"}, args...)) }}
}
