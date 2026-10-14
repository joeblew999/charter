package main

// charter access:setup: REMOTE
// Run by: mise run access:setup (tasks/project/tasks.toml)
func init() {
	commands["access:setup"] = command{"[email...]", "REMOTE: put Cloudflare Access in front of the Worker (API_URL): people log in with GitHub. Needs CLOUDFLARE_API_TOKEN, CLOUDFLARE_ACCOUNT_ID, CF_ACCESS_TEAM_DOMAIN and CF_ACCESS_GITHUB_IDP_ID (the environment, or fnox)", func(args []string) error { return access(append([]string{"setup"}, args...)) }}
}
