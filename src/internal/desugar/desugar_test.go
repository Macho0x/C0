package desugar_test

import (
	"testing"

	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/desugar"
	"goop.dev/compiler/internal/parser"
)

func parseExpr(t *testing.T, src string) ast.Expr {
	t.Helper()
	fullSrc := "module Test\nlet x = " + src
	mod, err := parser.Parse("test.goop", []byte(fullSrc))
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	ld := mod.Decls[0].(*ast.LetDecl)
	return ld.Bindings[0].Body
}

func TestDesugarFunction(t *testing.T) {
	expr := parseExpr(t, "function | 0 -> 1 | _ -> 2")
	if _, ok := expr.(*ast.FunctionExpr); !ok {
		t.Fatalf("expected FunctionExpr from parser, got %T", expr)
	}
	result := desugar.DesugarExpr(expr)
	fn, ok := result.(*ast.FunExpr)
	if !ok {
		t.Fatalf("expected FunExpr after desugar, got %T", result)
	}
	if len(fn.Params) != 1 || fn.Params[0].Name != "__fn_arg" {
		t.Fatalf("expected single __fn_arg param, got %+v", fn.Params)
	}
	m, ok := fn.Body.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected MatchExpr body, got %T", fn.Body)
	}
	if len(m.Arms) != 2 {
		t.Fatalf("expected 2 arms, got %d", len(m.Arms))
	}
}

func TestDesugarFunctionModule(t *testing.T) {
	src := `module Test
type status = Ok | Err of string

let handle = function
  | Err msg -> msg
  | Ok -> "ok"
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	ld := mod.Decls[1].(*ast.LetDecl)
	body := ld.Bindings[0].Body
	if _, ok := body.(*ast.FunExpr); !ok {
		t.Fatalf("expected FunExpr after function desugar, got %T", body)
	}
}

func TestDesugarPreservesOtherExprs(t *testing.T) {
	src := `module Test
let x = 1 + 2
`
	mod, err := parser.Parse("test.goop", []byte(src))
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

func TestDesugarWalksRefWhile(t *testing.T) {
	src := `module Test
let main () =
  let r = ref 0 in
  while !r < 3 do r := !r + 1 done
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)
	ld := mod.Decls[0].(*ast.LetDecl)
	body := ld.Bindings[0].Body
	letIn, ok := body.(*ast.LetInExpr)
	if !ok {
		t.Fatalf("expected LetInExpr, got %T", body)
	}
	if _, ok := letIn.Bindings[0].Body.(*ast.RefExpr); !ok {
		t.Fatalf("expected RefExpr binding, got %T", letIn.Bindings[0].Body)
	}
	if _, ok := letIn.Body.(*ast.WhileExpr); !ok {
		t.Fatalf("expected WhileExpr body, got %T", letIn.Body)
	}
}
