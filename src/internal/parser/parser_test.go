package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"c0.dev/compiler/internal/ast"
	"c0.dev/compiler/internal/parser"
)

// examplesDir is relative to the module root.
var examplesDir = "../../../docs/examples"

func TestParseHello(t *testing.T) {
	mod := mustParse(t, "hello.c0")
	if mod.Name != "Main" {
		t.Errorf("expected module name Main, got %q", mod.Name)
	}
	if len(mod.Decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(mod.Decls))
	}
	d, ok := mod.Decls[0].(*ast.LetDecl)
	if !ok {
		t.Fatalf("expected LetDecl, got %T", mod.Decls[0])
	}
	if len(d.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(d.Bindings))
	}
	if d.Bindings[0].Name != "main" {
		t.Errorf("expected binding name main, got %q", d.Bindings[0].Name)
	}
	// Body should be an AppExpr: print_line "Hello, C0!"
	app, ok := d.Bindings[0].Body.(*ast.AppExpr)
	if !ok {
		t.Fatalf("expected AppExpr body, got %T", d.Bindings[0].Body)
	}
	// Left side: print_line (ident) or Console.print_line (field access)
	ident, ok := app.Func.(*ast.IdentExpr)
	if !ok {
		// Backward compat: may be FieldAccessExpr for Console.print_line
		if field, ok2 := app.Func.(*ast.FieldAccessExpr); ok2 {
			if field.Field != "print_line" {
				t.Errorf("expected print_line, got %q", field.Field)
			}
		} else {
			t.Fatalf("expected IdentExpr or FieldAccessExpr, got %T", app.Func)
		}
	} else {
		if ident.Name != "print_line" {
			t.Errorf("expected print_line, got %q", ident.Name)
		}
	}
	// Right side: "Hello, C0!" string literal
	lit, ok := app.Arg.(*ast.LitExpr)
	if !ok {
		t.Fatalf("expected LitExpr arg, got %T", app.Arg)
	}
	if lit.Value != "Hello, C0!" {
		t.Errorf("expected 'Hello, C0!', got %v", lit.Value)
	}
}

func TestParseShapes(t *testing.T) {
	mod := mustParse(t, "shapes.c0")
	if mod.Name != "Shapes" {
		t.Errorf("expected Shapes, got %q", mod.Name)
	}
	if len(mod.Decls) != 3 {
		t.Fatalf("expected 3 decls (type shape, let area, let describe), got %d", len(mod.Decls))
	}

	// First decl: type shape = ADT
	td, ok := mod.Decls[0].(*ast.TypeDecl)
	if !ok {
		t.Fatalf("expected TypeDecl, got %T", mod.Decls[0])
	}
	if td.Name != "shape" {
		t.Errorf("expected type name shape, got %q", td.Name)
	}
	adt, ok := td.Kind.(*ast.ADTTypeKind)
	if !ok {
		t.Fatalf("expected ADTTypeKind, got %T", td.Kind)
	}
	if len(adt.Cases) != 3 {
		t.Errorf("expected 3 ADT cases, got %d", len(adt.Cases))
	}

	// Second decl: let area (s: shape) : float = match ...
	ld, ok := mod.Decls[1].(*ast.LetDecl)
	if !ok {
		t.Fatalf("expected LetDecl, got %T", mod.Decls[1])
	}
	b := ld.Bindings[0]
	if b.Name != "area" {
		t.Errorf("expected area, got %q", b.Name)
	}
	if len(b.Params) != 1 || b.Params[0].Name != "s" {
		t.Errorf("expected param s: shape")
	}
	match, ok := b.Body.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected MatchExpr body, got %T", b.Body)
	}
	if len(match.Arms) != 3 {
		t.Errorf("expected 3 match arms in area, got %d", len(match.Arms))
	}

	// Third decl: let describe (s: shape) : string = match ...
	ld2, ok := mod.Decls[2].(*ast.LetDecl)
	if !ok {
		t.Fatalf("expected LetDecl for describe, got %T", mod.Decls[2])
	}
	b2 := ld2.Bindings[0]
	if b2.Name != "describe" {
		t.Errorf("expected describe, got %q", b2.Name)
	}
	match2, ok := b2.Body.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected MatchExpr, got %T", b2.Body)
	}
	if len(match2.Arms) != 5 {
		t.Errorf("expected 5 match arms in describe, got %d", len(match2.Arms))
	}
	// First arm has a guard: when radius > 10.0
	if match2.Arms[0].Guard == nil {
		t.Error("expected guard on first arm of describe (when radius > 10.0)")
	}
	// Third arm has a guard: when width = height
	if match2.Arms[2].Guard == nil {
		t.Error("expected guard on third arm of describe (when width = height)")
	}
}

func TestParseResult(t *testing.T) {
	mod := mustParse(t, "result.c0")
	if mod.Name != "ResultExample" {
		t.Errorf("expected ResultExample, got %q", mod.Name)
	}
	if len(mod.Decls) != 6 {
		t.Fatalf("expected 6 decls, got %d", len(mod.Decls))
	}

	// type user = { id: int; name: string }
	tdUser, ok := mod.Decls[0].(*ast.TypeDecl)
	if !ok || tdUser.Name != "user" {
		t.Error("expected type user")
	}

	// type error = NotFound | InvalidInput of string
	tdErr, ok := mod.Decls[1].(*ast.TypeDecl)
	if !ok || tdErr.Name != "error" {
		t.Error("expected type error")
	}

	// let findUser ...
	ld, ok := mod.Decls[2].(*ast.LetDecl)
	if !ok || ld.Bindings[0].Name != "findUser" {
		t.Error("expected let findUser")
	}

	// let validateName ...
	ld2, ok := mod.Decls[3].(*ast.LetDecl)
	if !ok || ld2.Bindings[0].Name != "validateName" {
		t.Error("expected let validateName")
	}

	// let loadUser ... (body contains nested lets with ?)
	ld3, ok := mod.Decls[4].(*ast.LetDecl)
	if !ok || ld3.Bindings[0].Name != "loadUser" {
		t.Error("expected let loadUser")
	}
	// Verify ? operator is present in the nested expression
	body3 := ld3.Bindings[0].Body
	if letIn, ok := body3.(*ast.LetInExpr); ok {
		// body = let user = findUser id ? in (let validated = ... in Ok validated)
		// The binding body should contain a QuestionExpr
		if _, ok := letIn.Bindings[0].Body.(*ast.QuestionExpr); !ok {
			t.Errorf("expected QuestionExpr in nested let, got %T", letIn.Bindings[0].Body)
		}
	}

	// let getUserName ... (match on result)
	ld4, ok := mod.Decls[5].(*ast.LetDecl)
	if !ok || ld4.Bindings[0].Name != "getUserName" {
		t.Error("expected let getUserName")
	}
	match, ok := ld4.Bindings[0].Body.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected MatchExpr body, got %T", ld4.Bindings[0].Body)
	}
	if len(match.Arms) != 3 {
		t.Errorf("expected 3 match arms, got %d", len(match.Arms))
	}
}

func TestParseOrderbook(t *testing.T) {
	mod := mustParse(t, "orderbook.c0")
	if mod.Name != "Trading.OrderBook" {
		t.Errorf("expected Trading.OrderBook, got %q", mod.Name)
	}
	if len(mod.Decls) < 8 {
		t.Fatalf("expected at least 8 decls, got %d", len(mod.Decls))
	}

	// type side = Buy | Sell
	td, ok := mod.Decls[0].(*ast.TypeDecl)
	if !ok || td.Name != "side" {
		t.Error("expected type side")
	}

	// type order = { ... }
	td2, ok := mod.Decls[1].(*ast.TypeDecl)
	if !ok || td2.Name != "order" {
		t.Error("expected type order")
	}
	_, ok = td2.Kind.(*ast.RecordTypeKind)
	if !ok {
		t.Error("expected record type for order")
	}

	// let emptyBook : book = { bids = []; asks = [] }
	ld, ok := mod.Decls[4].(*ast.LetDecl)
	if !ok || ld.Bindings[0].Name != "emptyBook" {
		t.Error("expected let emptyBook")
	}

	// let rec insertBy ...
	ld2, ok := mod.Decls[6].(*ast.LetDecl)
	if !ok || !ld2.Rec {
		t.Error("expected rec let insertBy")
	}
	if ld2.Bindings[0].Name != "insertBy" {
		t.Errorf("expected insertBy, got %q", ld2.Bindings[0].Name)
	}
	// Body should be a match expression with cons pattern
	match, ok := ld2.Bindings[0].Body.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected MatchExpr, got %T", ld2.Bindings[0].Body)
	}
	if len(match.Arms) != 2 {
		t.Errorf("expected 2 match arms, got %d", len(match.Arms))
	}
	// Second arm should have a cons pattern: first :: rest
	if _, ok := match.Arms[1].Pattern.(*ast.ConsPattern); !ok {
		t.Errorf("expected ConsPattern in arm 1, got %T", match.Arms[1].Pattern)
	}

	// let addOrder ...
	ld3, ok := mod.Decls[7].(*ast.LetDecl)
	if !ok || ld3.Bindings[0].Name != "addOrder" {
		t.Error("expected let addOrder")
	}
	// Body should contain record update: { book with bids = ... }
	// (nested inside a match -> if branch)
	body := ld3.Bindings[0].Body
	m, ok := body.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected MatchExpr, got %T", body)
	}
	// Check that arms exist
	if len(m.Arms) != 2 {
		t.Errorf("expected 2 arms in addOrder match, got %d", len(m.Arms))
	}

	// let bestBid ...
	ld4, ok := mod.Decls[8].(*ast.LetDecl)
	if !ok || ld4.Bindings[0].Name != "bestBid" {
		t.Error("expected let bestBid")
	}
}

func TestLexHello(t *testing.T) {
	mod := mustParse(t, "hello.c0")
	_ = mod // just verify it parses; lexing is tested implicitly
}

func TestLexShapes(t *testing.T) {
	mod := mustParse(t, "shapes.c0")
	_ = mod
}

func TestLexResult(t *testing.T) {
	mod := mustParse(t, "result.c0")
	_ = mod
}

func TestLexOrderbook(t *testing.T) {
	mod := mustParse(t, "orderbook.c0")
	_ = mod
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func mustParse(t *testing.T, filename string) *ast.Module {
	t.Helper()
	path := filepath.Join(examplesDir, filename)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	mod, err := parser.Parse(filename, src)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return mod
}
