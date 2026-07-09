package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goop.dev/compiler/internal/config"
)

func TestGetUsage(t *testing.T) {
	if runGet(nil) == 0 {
		t.Fatal("expected failure with no args")
	}
}

func TestWriteTomlDependencies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "goop.toml")
	cfg := config.DefaultConfig()
	cfg.Dependencies = map[string]string{"github.com/acme/lib": "v1.0.0"}
	if err := writeTomlDependencies(path, cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "github.com/acme/lib") {
		t.Fatalf("missing dependency: %s", data)
	}
}
