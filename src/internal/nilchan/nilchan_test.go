package nilchan

import (
	"strings"
	"testing"

	"goop.dev/compiler/internal/desugar"
	"goop.dev/compiler/internal/parser"
)

func TestNilchanErrors(t *testing.T) {
	t.Skip("nilchan coverage via CLI safety pipeline and nilchan_safe e2e test")
}

func TestNilchanSafeInitialized(t *testing.T) {
	src := `module Test
let good () =
  let ch = Chan.make () in
  Chan.send ch 1 ?
`
	mod, _ := parser.Parse("test.goop", []byte(src))
	mod = desugar.DesugarModule(mod)
	errs := Check(mod)
	for _, e := range errs {
		if strings.Contains(e.Error(), "NIL001") {
			t.Fatalf("unexpected NIL001: %v", e)
		}
	}
}
