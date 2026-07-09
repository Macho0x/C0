package fmt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goop.dev/compiler/internal/parser"
)

func TestFormatGoMove(t *testing.T) {
	src := `module Main

let main () : unit with { async; io } =
  let mutable x = 1 in
  let dummy = go (move x) (fun () -> print_line (int_to_string x)) in
  print_line "done"
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out := FormatModule(mod)
	if !strings.Contains(out, "go (move x)") {
		t.Fatalf("missing go (move x) in:\n%s", out)
	}
}

func TestRoundTripSelected(t *testing.T) {
	names := []string{
		"go_move_test.goop",
		"guards_test.goop",
		"if_test.goop",
		"active_pattern_test.goop",
		"bool_test.goop",
	}
	for _, name := range names {
		path := filepath.Join("..", "..", "..", "tests", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		mod1, err := parser.Parse(path, data)
		if err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		out := FormatModule(mod1)
		mod2, err := parser.Parse(path, []byte(out))
		if err != nil {
			t.Fatalf("%s: re-parse: %v\n---\n%s", name, err, out)
		}
		if mod1.Name != mod2.Name || len(mod1.Decls) != len(mod2.Decls) {
			t.Fatalf("%s: module shape mismatch after round-trip", name)
		}
	}
}
