package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProjectConfigWalksParents(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "pkg", "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	toml := []byte("module_root = \"github.com/acme/demo\"\n")
	if err := os.WriteFile(filepath.Join(root, "goop.toml"), toml, 0644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(sub, "main.goop")
	if err := os.WriteFile(src, []byte("module main\nlet main () = ()\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := loadProjectConfig(src)
	if cfg.ModuleRoot != "github.com/acme/demo" {
		t.Fatalf("ModuleRoot=%q, want github.com/acme/demo", cfg.ModuleRoot)
	}
}
