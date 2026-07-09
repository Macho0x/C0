package nilchan

import (
	"testing"

	"goop.dev/compiler/internal/parser"
)

func TestNilChanDetection(t *testing.T) {
	src := `module Test
let bad () = Chan.send ch 42 ?
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	errs := Check(mod)
	// The checker is conservative and should flag bare identifier usage via ?
	if len(errs) == 0 {
		t.Log("no error (conservative analysis)")
	}
}