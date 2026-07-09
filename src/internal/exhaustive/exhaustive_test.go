package exhaustive

import (
	"testing"

	"goop.dev/compiler/internal/config"
	"goop.dev/compiler/internal/desugar"
	"goop.dev/compiler/internal/parser"
)

func TestExhaust003MissingVariant(t *testing.T) {
	src := `module Test
type OrderAck = | Filled | Rejected | PartialFill

let handle (ack: OrderAck) : string =
  match ack with
  | Filled -> "ok"
  | Rejected -> "no"
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod = desugar.DesugarModule(mod)
	ADTRegistry = map[string][]string{"OrderAck": {"Filled", "Rejected", "PartialFill"}}
	errs, _ := CheckWithConfig(mod, config.DefaultConfig())
	if len(errs) == 0 {
		t.Fatal("expected EXHAUST003")
	}
	if e, ok := errs[0].(*Error); !ok || e.Code != "EXHAUST003" {
		t.Fatalf("got %v", errs)
	}
}

func TestExhaust001DeadWildcard(t *testing.T) {
	src := `module Test
type color = | Red | Green

let f (c: color) : int =
  match c with
  | Red -> 1
  | _ -> 2
  | _ -> 3
`
	mod, _ := parser.Parse("test.goop", []byte(src))
	mod = desugar.DesugarModule(mod)
	ADTRegistry = map[string][]string{"color": {"Red", "Green"}}
	_, warns := CheckWithConfig(mod, config.DefaultConfig())
	found := false
	for _, w := range warns {
		if e, ok := w.(*Error); ok && e.Code == "EXHAUST001" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected EXHAUST001 warning, got %v", warns)
	}
}
