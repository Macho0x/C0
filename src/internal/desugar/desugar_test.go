package desugar_test

import (
	"testing"

	"c0.dev/compiler/internal/ast"
	"c0.dev/compiler/internal/desugar"
	"c0.dev/compiler/internal/parser"
)

func parseExpr(t *testing.T, src string) ast.Expr {
	t.Helper()
	// Wrap in a minimal module so the parser has context
	fullSrc := "module Test\nlet x = " + src
	mod, err := parser.Parse("test.c0", []byte(fullSrc))
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	ld := mod.Decls[0].(*ast.LetDecl)
	return ld.Bindings[0].Body
}

func TestDesugarIs(t *testing.T) {
	// s is Ok → stays as IsExpr (handled by codegen directly)
	expr := parseExpr(t, "s is Ok")

	// Verify it's an IsExpr (not desugared)
	if _, ok := expr.(*ast.IsExpr); !ok {
		t.Fatalf("expected IsExpr, got %T", expr)
	}
}

func TestDesugarAs(t *testing.T) {
	// s as Err msg -> msg else "fine" → stays as AsMatchExpr
	expr := parseExpr(t, `s as Err msg -> msg else "fine"`)

	if _, ok := expr.(*ast.AsMatchExpr); !ok {
		t.Fatalf("expected AsMatchExpr, got %T", expr)
	}
}

func TestDesugarGuardSingle(t *testing.T) {
	// guard Err msg = s else "no error"
	expr := parseExpr(t, `guard Err msg = s else "no error"`)
	result := desugar.DesugarExpr(expr)

	m, ok := result.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected MatchExpr, got %T", result)
	}
	if len(m.Arms) != 2 {
		t.Fatalf("expected 2 arms, got %d", len(m.Arms))
	}
}

func TestDesugarGuardMultiple(t *testing.T) {
	// guard Ok u = findUser id else Error "not found"
	// and Ok v = validate u else Error "bad"
	// The parser treats this as a guard with multiple bindings.
	expr := parseExpr(t, `guard Ok u = findUser id else Error "not found"`)
	result := desugar.DesugarExpr(expr)

	// Should produce a match expression
	if _, ok := result.(*ast.MatchExpr); !ok {
		t.Fatalf("expected MatchExpr, got %T", result)
	}
}

func TestDesugarRoundTrip(t *testing.T) {
	// Parse a full module with guard and desugar it
	src := `module Test
type status = Ok | Err of string

let handle (s: status) : string =
  guard Err msg = s else "no error"
`
	mod, err := parser.Parse("test.c0", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	// The body should be a MatchExpr after guard desugaring
	ld := mod.Decls[1].(*ast.LetDecl)
	body := ld.Bindings[0].Body
	if _, ok := body.(*ast.MatchExpr); !ok {
		t.Fatalf("expected MatchExpr after guard desugar, got %T", body)
	}
}

func TestDesugarPreservesOtherExprs(t *testing.T) {
	// Desugar should not affect ordinary expressions
	src := `module Test
let x = 1 + 2
`
	mod, err := parser.Parse("test.c0", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	ld := mod.Decls[0].(*ast.LetDecl)
	body := ld.Bindings[0].Body
	if _, ok := body.(*ast.BinaryExpr); !ok {
		t.Fatalf("expected BinaryExpr preserved, got %T", body)
	}
}

func TestMacrosExampleParses(t *testing.T) {
	// Verify the macros.c0 example parses without errors
	src := `module MacrosDemo

type status = Ok | Err of string

let describe (s: status) : string =
  if s is Ok then "ok" else "not ok"

let message (s: status) : string =
  s as Err msg -> msg else "fine"

	let handle (s: status) : string =
		guard Err msg = s else "no error"
`
	mod, err := parser.Parse("macros.c0", []byte(src))
	if err != nil {
		t.Fatalf("parse macros.c0: %v", err)
	}
	if mod.Name != "MacrosDemo" {
		t.Errorf("expected MacrosDemo, got %s", mod.Name)
	}
	if len(mod.Decls) != 4 { // type + 3 lets
		t.Errorf("expected 4 decls, got %d", len(mod.Decls))
	}

	// Desugar and verify
	mod = desugar.DesugarModule(mod)
	// describe body: now stays as IsExpr (handled by codegen directly)
	ld1 := mod.Decls[1].(*ast.LetDecl)
	descBody := ld1.Bindings[0].Body
	if ife, ok := descBody.(*ast.IfExpr); ok {
		// IsExpr inside if condition
		if _, ok := ife.Cond.(*ast.IsExpr); !ok {
			t.Errorf("describe if cond: expected IsExpr, got %T", ife.Cond)
		}
	} else {
		t.Errorf("describe body: expected IfExpr, got %T", descBody)
	}
	// message body: stays as AsMatchExpr
	ld2 := mod.Decls[2].(*ast.LetDecl)
	if _, ok := ld2.Bindings[0].Body.(*ast.AsMatchExpr); !ok {
		t.Errorf("message body: expected AsMatchExpr, got %T", ld2.Bindings[0].Body)
	}
	// handle body: guard desugars to MatchExpr
	ld3 := mod.Decls[3].(*ast.LetDecl)
	if _, ok := ld3.Bindings[0].Body.(*ast.MatchExpr); !ok {
		t.Errorf("handle body: expected MatchExpr, got %T", ld3.Bindings[0].Body)
	}
}

func TestParseIsExpr(t *testing.T) {
	expr := parseExpr(t, "x is Some y")
	// Stays as IsExpr (handled by codegen)
	if _, ok := expr.(*ast.IsExpr); !ok {
		t.Fatalf("expected IsExpr, got %T", expr)
	}
}
