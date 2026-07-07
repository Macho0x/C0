package refine

import (
	"strings"
	"testing"

	"c0.dev/compiler/internal/ast"
	"c0.dev/compiler/internal/parser"
	"c0.dev/compiler/internal/desugar"
	"c0.dev/compiler/internal/typecheck"
	"c0.dev/compiler/internal/typeinfo"
)

// testMod parses and type-checks a C0 source snippet and returns the module and type map.
func testMod(t *testing.T, src string) (*ast.Module, typeinfo.TypeMap) {
	t.Helper()
	mod, err := parser.Parse("test.c0", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	tm, _, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Logf("type error: %v", e)
		}
		t.Fatalf("typecheck failed")
	}
	return mod, tm
}

func TestVC_ProvenRefinement(t *testing.T) {
	src := `module Test

let safeDiv (a: int) (b: int where b <> 0) : int = a / b

let compute (x: int) (y: int) : int =
  if y <> 0 then
    safeDiv x y
  else
    0
`
	mod, tm := testMod(t, src)
	proven, warnings, errs := CheckRefinements(mod, tm)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("unexpected error: %v", e)
		}
	}

	// We expect the safeDiv call in the then-branch to be proven
	if len(proven) == 0 {
		t.Error("expected at least one proven call site")
	}

	// There should be no warnings for this module (the only call is proven)
	for _, w := range warnings {
		t.Logf("warning: %v", w)
	}
}

func TestVC_UnprovenWithWarning(t *testing.T) {
	src := `module Test

let safeDiv (a: int) (b: int where b <> 0) : int = a / b

let risky (x: int) (y: int) : int =
  safeDiv x y
`
	mod, tm := testMod(t, src)
	proven, warnings, errs := CheckRefinements(mod, tm)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("unexpected error: %v", e)
		}
	}

	// The call in risky has no context, so it should be unproven
	if len(proven) > 0 {
		t.Error("expected no proven call sites for risky")
	}

	// Should have at least one warning about unproven refinement
	if len(warnings) == 0 {
		t.Error("expected warning about unproven refinement")
	}
	for _, w := range warnings {
		if !strings.Contains(w.Error(), "could not prove") {
			t.Errorf("warning should contain 'could not prove': %v", w)
		}
	}
}

func TestVC_DisprovenRefinement(t *testing.T) {
	src := `module Test

let safeDiv (a: int) (b: int where b <> 0) : int = a / b

let bad (x: int) : int =
  if x = 0 then
    safeDiv 100 x
  else
    0
`
	mod, tm := testMod(t, src)
	_, warnings, errs := CheckRefinements(mod, tm)

	// The call safeDiv 100 x in the then-branch has x = 0 as constraint
	// x <> 0 with x = 0 should be Disproven
	if len(errs) == 0 {
		t.Error("expected refinement violation error")
	}
	for _, e := range errs {
		if !strings.Contains(e.Error(), "refinement violated") && !strings.Contains(e.Error(), "cannot satisfy") {
			t.Errorf("error should mention refinement violation: %v", e)
		}
	}
	_ = warnings
}
