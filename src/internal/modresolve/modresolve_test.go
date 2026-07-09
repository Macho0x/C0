package modresolve_test

import (
	"path/filepath"
	"testing"

	"goop.dev/compiler/internal/config"
	"goop.dev/compiler/internal/modresolve"
)

func TestResolveStdIO(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	cfg := config.DefaultConfig()
	r := modresolve.New(cfg, nil, root)
	res, err := r.ResolveGoopPath("std.io")
	if err != nil {
		t.Fatal(err)
	}
	if res.GoImportPath != "github.com/Macho0x/Goop/std/io" {
		t.Errorf("go path: %s", res.GoImportPath)
	}
	if res.SourceFile == "" {
		t.Fatal("expected source file")
	}
}

func TestExportNamesFiltersPrivate(t *testing.T) {
	names := modresolve.ExportNames(nil)
	if names != nil {
		t.Errorf("expected nil for nil module, got %v", names)
	}
}

func TestMissingModule(t *testing.T) {
	cfg := &config.Config{Mappings: map[string]string{}}
	r := modresolve.New(cfg, nil, "/nonexistent")
	res, err := r.ResolveGoopPath("unknown.logical")
	if err != nil {
		t.Fatal(err)
	}
	if res.SourceFile != "" {
		t.Fatal("expected no source for unknown module")
	}
}

func TestLockfileOverride(t *testing.T) {
	lock := &config.Lockfile{}
	lock.Upsert(config.LockModule{
		Path:    "github.com/acme/lib",
		Version: "v1.0.0",
		Source:  "github.com/acme/override",
	})
	cfg := config.DefaultConfig()
	r := modresolve.New(cfg, lock, "")
	res, err := r.ResolveGoopPath("github.com/acme/lib")
	if err != nil {
		t.Fatal(err)
	}
	if res.GoImportPath != "github.com/acme/lib" {
		t.Errorf("got %s", res.GoImportPath)
	}
}
