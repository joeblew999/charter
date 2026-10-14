package main

// charter access:delete: REMOTE
// Run by: mise run access:delete (tasks/project/tasks.toml)
func init() {
	commands["access:delete"] = command{"", "REMOTE: delete the Worker's Access application and its machines' service tokens", func(args []string) error { return access(append([]string{"delete"}, args...)) }}
}
