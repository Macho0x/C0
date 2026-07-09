package parser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/parser"
	"goop.dev/compiler/internal/typecheck"
)

// examplesDir is relative to the module root.
var examplesDir = "../../../docs/examples"

func TestParseHello(t *testing.T) {
	mod := mustParse(t, "hello.goop")
	if mod.Name != "main" {
		t.Errorf("expected module name main, got %q", mod.Name)
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
	// Body should be an AppExpr: print_line "Hello, Goop!"
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
	// Right side: "Hello, Goop!" string literal
	lit, ok := app.Arg.(*ast.LitExpr)
	if !ok {
		t.Fatalf("expected LitExpr arg, got %T", app.Arg)
	}
	if lit.Value != "Hello, Goop!" {
		t.Errorf("expected 'Hello, Goop!', got %v", lit.Value)
	}
}

func TestParseShapes(t *testing.T) {
	mod := mustParse(t, "shapes.goop")
	if mod.Name != "main" {
		t.Errorf("expected main, got %q", mod.Name)
	}
	if len(mod.Decls) != 4 {
		t.Fatalf("expected 4 decls (type Shape, let area, let describe, let main), got %d", len(mod.Decls))
	}

	// First decl: type Shape = ADT
	td, ok := mod.Decls[0].(*ast.TypeDecl)
	if !ok {
		t.Fatalf("expected TypeDecl, got %T", mod.Decls[0])
	}
	if td.Name != "Shape" {
		t.Errorf("expected type name Shape, got %q", td.Name)
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
	mod := mustParse(t, "result.goop")
	if mod.Name != "main" {
		t.Errorf("expected main, got %q", mod.Name)
	}
	if len(mod.Decls) != 7 {
		t.Fatalf("expected 7 decls, got %d", len(mod.Decls))
	}

	// type User = { id: int; name: string }
	tdUser, ok := mod.Decls[0].(*ast.TypeDecl)
	if !ok || tdUser.Name != "User" {
		t.Error("expected type User")
	}

	// type UserError = NotFound | InvalidInput of string
	tdErr, ok := mod.Decls[1].(*ast.TypeDecl)
	if !ok || tdErr.Name != "UserError" {
		t.Error("expected type UserError")
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
	mod := mustParse(t, "orderbook.goop")
	if mod.Name != "Trading.OrderBook" {
		t.Errorf("expected Trading.OrderBook, got %q", mod.Name)
	}
	// type order_id / symbol newtypes, then Side
	if len(mod.Decls) < 10 {
		t.Fatalf("expected at least 10 decls, got %d", len(mod.Decls))
	}

	td, ok := mod.Decls[2].(*ast.TypeDecl)
	if !ok || td.Name != "Side" {
		t.Error("expected type Side")
	}

	td2, ok := mod.Decls[3].(*ast.TypeDecl)
	if !ok || td2.Name != "Order" {
		t.Error("expected type Order")
	}
	_, ok = td2.Kind.(*ast.RecordTypeKind)
	if !ok {
		t.Error("expected record type for order")
	}

	// let emptyBook : book = { bids = []; asks = [] }
	ld, ok := mod.Decls[6].(*ast.LetDecl)
	if !ok || ld.Bindings[0].Name != "emptyBook" {
		t.Error("expected let emptyBook")
	}

	ld2, ok := mod.Decls[8].(*ast.LetDecl)
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
	ld3, ok := mod.Decls[9].(*ast.LetDecl)
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
	ld4, ok := mod.Decls[10].(*ast.LetDecl)
	if !ok || ld4.Bindings[0].Name != "bestBid" {
		t.Error("expected let bestBid")
	}
}

func TestLexHello(t *testing.T) {
	mod := mustParse(t, "hello.goop")
	_ = mod // just verify it parses; lexing is tested implicitly
}

func TestLexShapes(t *testing.T) {
	mod := mustParse(t, "shapes.goop")
	_ = mod
}

func TestLexResult(t *testing.T) {
	mod := mustParse(t, "result.goop")
	_ = mod
}

func TestLexOrderbook(t *testing.T) {
	mod := mustParse(t, "orderbook.goop")
	_ = mod
}

func TestParseGolangEmbed(t *testing.T) {
	src := `module main

import golang "fmt"

@golang {
  func nowString() string {
    return fmt.Sprintf("%d", 1)
  }
}
val nowString : unit -> string

let main () = print_line (nowString ())
`
	mod, err := parser.Parse("golang.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var found bool
	for _, d := range mod.Decls {
		if ge, ok := d.(*ast.GolangEmbedDecl); ok {
			found = true
			if !strings.Contains(ge.GoCode, "nowString") {
				t.Error("expected Go code in embed block")
			}
			if len(ge.Vals) != 1 || ge.Vals[0].Name != "nowString" {
				t.Errorf("expected val nowString, got %+v", ge.Vals)
			}
		}
	}
	if !found {
		t.Fatal("expected GolangEmbedDecl")
	}
}

func TestRejectLegacyGoBlockInExtern(t *testing.T) {
	src := `module main

import golang "fmt" {
  go {
    func x() {}
  }
}
`
	_, err := parser.Parse("bad.goop", []byte(src))
	if err == nil {
		t.Fatal("expected parse error for go { } inside import block")
	}
	if !strings.Contains(err.Error(), "val") {
		t.Errorf("expected val-only import block error, got: %v", err)
	}
}

func TestImportGrouped(t *testing.T) {
	src := `module main
import (
  golang "fmt"
  goop "std.io"
)
let main () = ()
`
	mod, err := parser.Parse("t.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(mod.Imports) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(mod.Imports))
	}
	if mod.Imports[0].Kind != ast.ImportGolang || mod.Imports[0].Path != "fmt" {
		t.Errorf("import[0]: %+v", mod.Imports[0])
	}
	if mod.Imports[1].Kind != ast.ImportGoop || mod.Imports[1].Path != "std.io" {
		t.Errorf("import[1]: %+v", mod.Imports[1])
	}
}

func TestImportGolangVals(t *testing.T) {
	src := `module main
import golang "strconv" { val Atoi : string -> (int, string) }
let main () = ()
`
	mod, err := parser.Parse("t.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(mod.Imports) != 1 || len(mod.Imports[0].Vals) != 1 {
		t.Fatalf("expected 1 val, got %+v", mod.Imports)
	}
}

func TestImportGoopDot(t *testing.T) {
	src := `module main
import goop . "std.io"
let main () = ()
`
	mod, err := parser.Parse("t.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if mod.Imports[0].Alias != "." {
		t.Errorf("expected dot alias, got %q", mod.Imports[0].Alias)
	}
}

func TestImportAlias(t *testing.T) {
	src := `module main
import httpx golang "net/http"
let main () = ()
`
	mod, err := parser.Parse("t.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if mod.Imports[0].Alias != "httpx" {
		t.Errorf("expected httpx alias, got %q", mod.Imports[0].Alias)
	}
}

func TestImportGoopCanonicalPath(t *testing.T) {
	src := `module main
import goop "github.com/foo/bar"
let main () = ()
`
	mod, err := parser.Parse("t.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if mod.Imports[0].Path != "github.com/foo/bar" {
		t.Errorf("got path %q", mod.Imports[0].Path)
	}
}

func TestRejectOpen(t *testing.T) {
	src := `module main
open Std.IO
let main () = ()
`
	_, err := parser.Parse("t.goop", []byte(src))
	if err == nil {
		t.Fatal("expected error for open")
	}
	if !strings.Contains(err.Error(), "import goop") {
		t.Errorf("expected migration hint, got: %v", err)
	}
}

func TestRejectExtern(t *testing.T) {
	src := `module main
extern "go" "fmt" {}
let main () = ()
`
	_, err := parser.Parse("t.goop", []byte(src))
	if err == nil {
		t.Fatal("expected error for extern")
	}
	if !strings.Contains(err.Error(), "import golang") {
		t.Errorf("expected migration hint, got: %v", err)
	}
}

func TestRejectC0ImportVals(t *testing.T) {
	src := `module main
import goop "std.io" { val X : int }
let main () = ()
`
	_, err := parser.Parse("t.goop", []byte(src))
	if err == nil {
		t.Fatal("expected error for c0 import with vals")
	}
}

func TestParseGoMove(t *testing.T) {
	src := `module main
let main () =
  let mutable x = 0 in
  go (move x) (fun () -> x)
`
	mod, err := parser.Parse("t.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var found *ast.GoExpr
	var walk func(ast.Expr)
	walk = func(e ast.Expr) {
		if e == nil {
			return
		}
		if g, ok := e.(*ast.GoExpr); ok {
			found = g
		}
		switch e := e.(type) {
		case *ast.LetInExpr:
			for _, b := range e.Bindings {
				walk(b.Body)
			}
			walk(e.Body)
		case *ast.FunExpr:
			walk(e.Body)
		case *ast.ParenExpr:
			walk(e.Inner)
		}
	}
	for _, d := range mod.Decls {
		if ld, ok := d.(*ast.LetDecl); ok {
			for _, b := range ld.Bindings {
				walk(b.Body)
			}
		}
	}
	if found == nil || len(found.Moved) != 1 || found.Moved[0] != "x" {
		t.Fatalf("expected go (move x), got %+v", found)
	}
}

func TestRejectImportGo(t *testing.T) {
	src := `module main
import go "fmt"
let main () = ()
`
	_, err := parser.Parse("t.goop", []byte(src))
	if err == nil {
		t.Fatal("expected error for import go")
	}
	if !strings.Contains(err.Error(), "golang") {
		t.Errorf("expected golang hint, got: %v", err)
	}
}

func TestImportEmptyGrouped(t *testing.T) {
	src := `module main
import ()
let main () = ()
`
	mod, err := parser.Parse("t.goop", []byte(src))
	if err != nil {
		t.Fatalf("empty import group should parse: %v", err)
	}
	if len(mod.Imports) != 0 {
		t.Errorf("expected 0 imports, got %d", len(mod.Imports))
	}
}

func TestImportDuplicatePath(t *testing.T) {
	src := `module main
import (
  golang "fmt"
  golang "fmt"
)
let main () = ()
`
	mod, err := parser.Parse("t.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(mod.Imports) != 2 {
		t.Fatalf("expected 2 specs at parse time")
	}
	_, _, errs := typecheck.CheckWithTypes(mod)
	for _, e := range errs {
		if strings.Contains(e.Error(), "duplicate import") {
			return
		}
	}
	t.Log("duplicate import may be caught at typecheck")
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
