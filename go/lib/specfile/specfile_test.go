package specfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestASpecIsWrittenIndentedAndCheckedAgainstTheFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "openapi.json")
	spec := func(server string) ([]byte, error) { return []byte(`{"servers":"` + server + `"}`), nil }
	if err := one(spec, file, "https://a.example", false); err != nil {
		t.Fatal(err)
	}
	if written, _ := os.ReadFile(file); string(written) != "{\n  \"servers\": \"https://a.example\"\n}\n" {
		t.Fatalf("written: %q", written)
	}
	if err := one(spec, file, "https://a.example", true); err != nil {
		t.Errorf("checking what was just written: %v", err)
	}
	// The contract (here: the server) changed and the file was not written again.
	if err := one(spec, file, "https://b.example", true); err == nil || !strings.Contains(err.Error(), "is stale") {
		t.Errorf("a stale file: %v", err)
	}
	if written, _ := os.ReadFile(file); !strings.Contains(string(written), "a.example") {
		t.Errorf("-check wrote the file: %q", written)
	}
	broken := func(string) ([]byte, error) { return nil, errors.New("no contract") }
	if err := one(broken, file, "https://a.example", false); err == nil {
		t.Error("a spec that fails was written")
	}
}
