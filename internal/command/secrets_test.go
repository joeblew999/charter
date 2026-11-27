package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joeblew999/charter/internal/project"
)

// A secret is the app's when the app's own fnox.toml names it, for the Worker itself or for an environment's
// profile; and where a provider keeps it carries charter's name, the Worker's and the environment's, so no two
// apps and no two environments share one.
func TestAnAppsSecretsAreItsOwn(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "fnox.toml"), []byte(`[secrets]
ADMIN_TOKEN= { provider = "keychain", value = "CHARTER_MY_APP_ADMIN_TOKEN", if_missing = "ignore" }

[profiles.staging.secrets]
WRITE_TOKEN= { provider = "keychain", value = "CHARTER_MY_APP_STAGING_WRITE_TOKEN" }
`), 0o644)
	p := &project.Project{Root: root}
	p.Worker.Name = "my-app"
	for _, c := range []struct {
		env, name string
		want      bool
	}{
		{"", "ADMIN_TOKEN", true}, {"", "WRITE_TOKEN", false}, {"staging", "WRITE_TOKEN", true},
		{"staging", "ADMIN_TOKEN", false}, {"", "READ_TOKEN", false},
	} {
		if got := declared(p, c.env, c.name); got != c.want {
			t.Errorf("declared(%q, %s) = %v, want %v", c.env, c.name, got, c.want)
		}
	}
	if got := stored(p, "", "ADMIN_TOKEN"); got != "CHARTER_MY_APP_ADMIN_TOKEN" {
		t.Errorf("stored: %s", got)
	}
	if got := stored(p, "staging", "ADMIN_TOKEN"); got != "CHARTER_MY_APP_STAGING_ADMIN_TOKEN" {
		t.Errorf("stored for an environment: %s", got)
	}
}
