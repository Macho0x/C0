package codegen_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"c0.dev/compiler/internal/ast"
	"c0.dev/compiler/internal/codegen"
	"c0.dev/compiler/internal/config"
	"c0.dev/compiler/internal/parser"
)

func TestSourceMapGenerated(t *testing.T) {
	path := filepath.Join(examplesDir, "hello.c0")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	mod, err := parser.Parse("hello.c0", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	gen := codegen.NewGenerator("hello.c0", config.DefaultConfig())
	_, err = gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	sm := gen.SourceMap()
	if sm == nil {
		t.Fatal("expected non-nil SourceMap")
	}
	if sm.Version != 3 {
		t.Errorf("expected version 3, got %d", sm.Version)
	}
	if sm.Source != "hello.c0" {
		t.Errorf("expected source 'hello.c0', got %q", sm.Source)
	}
	if sm.Generated != "main.go" {
		t.Errorf("expected generated 'main.go', got %q", sm.Generated)
	}
	if len(sm.Mappings) == 0 {
		t.Fatal("expected at least one mapping")
	}

	// Verify first mapping has reasonable values
	first := sm.Mappings[0]
	if first.C0Line < 1 {
		t.Errorf("expected C0Line >= 1, got %d", first.C0Line)
	}
	if first.GoLine < 1 {
		t.Errorf("expected GoLine >= 1, got %d", first.GoLine)
	}
}

func TestSourceMapWrite(t *testing.T) {
	path := filepath.Join(examplesDir, "hello.c0")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	mod, err := parser.Parse("hello.c0", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	gen := codegen.NewGenerator("hello.c0", config.DefaultConfig())
	_, err = gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	sm := gen.SourceMap()

	// Write to a buffer and parse back
	var buf strings.Builder
	if err := sm.Write(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}

	var parsed codegen.SourceMap // can't access unexported fields directly, use json
	if err := json.Unmarshal([]byte(buf.String()), &parsed); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if parsed.Version != 3 {
		t.Errorf("round-trip version: got %d", parsed.Version)
	}
	if len(parsed.Mappings) == 0 {
		t.Fatal("round-trip: expected mappings")
	}
}

func TestCompileWritesMapFile(t *testing.T) {
	path := filepath.Join(examplesDir, "hello.c0")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	mod, err := parser.Parse("hello.c0", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	gen := codegen.NewGenerator("hello.c0", config.DefaultConfig())
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	tmpDir := t.TempDir()
	goPath := filepath.Join(tmpDir, "main.go")
	mapPath := goPath + ".map.json"

	// Write Go file
	if err := os.WriteFile(goPath, []byte(goSrc), 0644); err != nil {
		t.Fatal(err)
	}

	// Write source map
	sm := gen.SourceMap()
	if sm == nil {
		t.Fatal("expected source map")
	}
	f, err := os.Create(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := sm.Write(f); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Verify map file exists and has content
	mapData, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatalf("read map: %v", err)
	}
	if !strings.Contains(string(mapData), `"version"`) {
		t.Error("map file missing version field")
	}
	if !strings.Contains(string(mapData), `"hello.c0"`) {
		t.Error("map file missing source reference")
	}
}

func mustParseMod(t *testing.T, rel string) *ast.Module {
	t.Helper()
	path := filepath.Join(examplesDir, rel)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	mod, err := parser.Parse(rel, src)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return mod
}
