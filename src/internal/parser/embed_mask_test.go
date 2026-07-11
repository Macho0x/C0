package parser

import (
	"bytes"
	"testing"
)

func TestMaskLangEmbedBodiesPreservesNewlines(t *testing.T) {
	src := []byte("module m\n\n@[go] {\n  func(*T) {}\n}\n")
	masked := maskLangEmbedBodies(src)
	if len(masked) != len(src) {
		t.Fatalf("length changed %d -> %d", len(src), len(masked))
	}
	if bytes.Count(masked, []byte("\n")) != bytes.Count(src, []byte("\n")) {
		t.Fatal("newline count changed")
	}
	if bytes.Contains(masked, []byte("(*")) {
		t.Fatalf("(* still visible to lexer: %q", masked)
	}
}
