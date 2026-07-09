package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goop.dev/compiler/internal/config"
)

func TestLoadLockfile(t *testing.T) {
	data := `[[module]]
path = "github.com/acme/lib"
version = "v1.2.3"
source = "github.com/acme/lib"
`
	path := filepath.Join(t.TempDir(), "goop.lock")
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	lf, err := config.LoadLockfile(path)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := lf.Lookup("github.com/acme/lib")
	if !ok || m.Version != "v1.2.3" {
		t.Fatalf("lookup: %+v ok=%v", m, ok)
	}
	out := lf.Format()
	if !strings.Contains(out, "v1.2.3") {
		t.Errorf("format: %s", out)
	}
}
