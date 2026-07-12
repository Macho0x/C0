package main

import (
	"os"
	"path/filepath"
	"testing"

	"goop.dev/compiler/internal/config"
)

func TestURIToPathDecodesSpaces(t *testing.T) {
	got := uriToPath("file:///home/redvelvet/Documents/Projects/Goop%20Tree%20Logger/treelog.goop")
	want := "/home/redvelvet/Documents/Projects/Goop Tree Logger/treelog.goop"
	if got != want {
		t.Fatalf("uriToPath spaces: got %q want %q", got, want)
	}
}

func TestURIToPathAlreadyDecoded(t *testing.T) {
	in := "file:///tmp/plain/file.goop"
	got := uriToPath(in)
	want := "/tmp/plain/file.goop"
	if got != want {
		t.Fatalf("uriToPath plain: got %q want %q", got, want)
	}
}

func TestURIToPathNestedEncoded(t *testing.T) {
	got := uriToPath("file:///tmp/a%20b/c%20d.goop")
	want := "/tmp/a b/c d.goop"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLoadProjectConfigSpaceInPath(t *testing.T) {
	root := t.TempDir()
	// Simulate a project path that would appear after URI decode.
	proj := filepath.Join(root, "Goop Tree Logger")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	toml := []byte("module_root = \"github.com/Macho0x/treelog\"\n")
	if err := os.WriteFile(filepath.Join(proj, "goop.toml"), toml, 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(proj, "examples", "example.goop")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := loadProjectConfig(src)
	if cfg == nil || cfg.ModuleRoot != "github.com/Macho0x/treelog" {
		t.Fatalf("expected module_root from spaced path, got %+v", cfg)
	}
	_ = config.DefaultConfig()
}
