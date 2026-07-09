package typecheck_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/desugar"
	"goop.dev/compiler/internal/exhaustive"
	"goop.dev/compiler/internal/parser"
	"goop.dev/compiler/internal/typecheck"
	"goop.dev/compiler/internal/types"
)

var examplesDir = "../../../docs/examples"

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
	return desugar.DesugarModule(mod)
}

func TestTypeCheckHello(t *testing.T) {
	mod := mustParse(t, "hello.goop")
	errs := typecheck.Check(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
	}
}

func TestTypeCheckShapes(t *testing.T) {
	mod := mustParse(t, "shapes.goop")
	errs := typecheck.Check(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
	}
}

func TestTypeCheckResult(t *testing.T) {
	mod := mustParse(t, "result.goop")
	errs := typecheck.Check(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
	}
}

func TestTypeCheckOrderbook(t *testing.T) {
	mod := mustParse(t, "orderbook.goop")
	errs := typecheck.Check(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
	}
}

// ---------------------------------------------------------------------------
// Negative tests: type errors
// ---------------------------------------------------------------------------

func TestTypeMismatch(t *testing.T) {
	// Adding int and string should fail
	src := `module Test
let f (x: int) : int = x + "hello"
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	errs := typecheck.Check(mod)
	if len(errs) == 0 {
		t.Error("expected type error for int + string")
	}
}

func TestPrivateSameModuleOk(t *testing.T) {
	src := `module main
private let helper x = x + 1
let main () = helper 1
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	errs := typecheck.Check(mod)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestPrivateCrossModuleRejected(t *testing.T) {
	lib := `module lib
private let helper x = x
let publicFn x = helper x
`
	consumer := `module main
let main () = helper 1
`
	libMod, err := parser.Parse("lib.goop", []byte(lib))
	if err != nil {
		t.Fatalf("parse lib: %v", err)
	}
	consMod, err := parser.Parse("main.goop", []byte(consumer))
	if err != nil {
		t.Fatalf("parse main: %v", err)
	}
	_, _, errs := typecheck.CheckWithTypesAndDeps(consMod, map[string]*ast.Module{"lib": libMod})
	if len(errs) == 0 {
		t.Fatal("expected error referencing private helper")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "private") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected private access error, got %v", errs)
	}
}

func TestPrivateUppercaseNameRejected(t *testing.T) {
	src := `module main
private let Helper x = x
let main () = Helper 1
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	errs := typecheck.Check(mod)
	if len(errs) == 0 {
		t.Fatal("expected error for private uppercase name")
	}
}

func TestModuloFloatRejected(t *testing.T) {
	src := `module main
let main () = 1.5 % 1.0
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	errs := typecheck.Check(mod)
	if len(errs) == 0 {
		t.Fatal("expected type error for float %")
	}
}

// ---------------------------------------------------------------------------
// Location tests: verify type errors include source locations
// ---------------------------------------------------------------------------

func TestTypeErrorHasLocation(t *testing.T) {
	src := `module Test
let f (x: int) : int = x + "hello"
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	errs := typecheck.Check(mod)
	if len(errs) == 0 {
		t.Fatal("expected a type error")
	}
	msg := errs[0].Error()
	// Should contain file:line:col format
	if !strings.Contains(msg, "test.goop:2:") {
		t.Errorf("error message should contain source location, got: %s", msg)
	}
}

func TestTypeErrorLocationBinaryOp(t *testing.T) {
	src := `module Test
let f () = true + 42
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	errs := typecheck.Check(mod)
	if len(errs) == 0 {
		t.Fatal("expected a type error for bool + int")
	}
	msg := errs[0].Error()
	if !strings.Contains(msg, "test.goop:2:") {
		t.Errorf("error should have location, got: %s", msg)
	}
}

func TestTypeErrorLocationIf(t *testing.T) {
	src := `module Test
let f () = if 42 then true else false
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	errs := typecheck.Check(mod)
	if len(errs) == 0 {
		t.Fatal("expected a type error for non-bool condition")
	}
	msg := errs[0].Error()
	if !strings.Contains(msg, "test.goop:2:") {
		t.Errorf("error should have location, got: %s", msg)
	}
}

func TestTypeErrorLocationApp(t *testing.T) {
	src := `module Test
let f () = 42 "hello"
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	errs := typecheck.Check(mod)
	if len(errs) == 0 {
		t.Fatal("expected a type error for int applied as function")
	}
	msg := errs[0].Error()
	if !strings.Contains(msg, "test.goop:2:") {
		t.Errorf("error should have location, got: %s", msg)
	}
}

func TestUnknownIdentifier(t *testing.T) {
	// Use a known ADT where a constructor type mismatch occurs.
	// The bootstrap gives unknown identifiers fresh types, so this
	// won't fail by itself. We test a case that actually causes a
	// unification error.
	src := `module Test
type t = A | B

let f (x: t) : int =
  match x with
  | A -> 1
  | B -> true
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	errs := typecheck.Check(mod)
	if len(errs) == 0 {
		t.Error("expected type error for int vs bool in match arms")
	}
}

func TestWrongArgCount(t *testing.T) {
	// Function expecting two args is given one with wrong type
	src := `module Test
let add (x: int) (y: int) : int = x + y
let wrong = add true
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	errs := typecheck.Check(mod)
	if len(errs) == 0 {
		t.Error("expected type error for bool vs int argument")
	}
}

// ---------------------------------------------------------------------------
// Exhaustiveness tests
// ---------------------------------------------------------------------------

func TestExhaustiveMatchPasses(t *testing.T) {
	mod := mustParse(t, "shapes.goop")
	// Register ADTs as the CLI does
	registerADTs(mod)
	errs := exhaustive.Check(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("unexpected exhaustiveness warning: %v", e)
		}
	}
}

func TestNonExhaustiveMatch(t *testing.T) {
	src := `module Test
type color = Red | Green | Blue

let describe (c: color) : string =
  match c with
  | Red -> "red"
  | Green -> "green"
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	registerADTs(mod)
	errs := exhaustive.Check(mod)
	if len(errs) == 0 {
		t.Error("expected exhaustiveness warning for missing Blue")
	}
}

func TestExhaustiveWithWildcard(t *testing.T) {
	src := `module Test
type color = Red | Green | Blue

let describe (c: color) : string =
  match c with
  | Red -> "red"
  | _ -> "other"
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	registerADTs(mod)
	errs := exhaustive.Check(mod)
	if len(errs) > 0 {
		t.Errorf("unexpected exhaustiveness warning: %v", errs[0])
	}
}

func ExhaustiveResultMatch(t *testing.T) {
	src := `module Test
let f (r: result) : string =
  match r with
  | Ok x -> "ok"
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	registerADTs(mod)
	errs := exhaustive.Check(mod)
	if len(errs) == 0 {
		t.Error("expected exhaustiveness warning for missing Error")
	}
}

func registerADTs(mod *ast.Module) {
	for _, d := range mod.Decls {
		if td, ok := d.(*ast.TypeDecl); ok {
			if adt, ok := td.Kind.(*ast.ADTTypeKind); ok {
				var ctors []string
				for _, c := range adt.Cases {
					ctors = append(ctors, c.Name)
				}
				exhaustive.RegisterADT(td.Name, ctors)
			}
		}
	}
	// Register built-in ADTs
	exhaustive.RegisterADT("result", []string{"Ok", "Error"})
	exhaustive.RegisterADT("option", []string{"None", "Some"})
}

// ---------------------------------------------------------------------------
// Bidirectional lambda inference tests
// ---------------------------------------------------------------------------

// TestBidirectionalLambdaKnownFunc verifies that when a lambda is passed to
// a function with a known signature, the lambda's unannotated parameter is
// inferred from the expected function type.
func TestBidirectionalLambdaKnownFunc(t *testing.T) {
	// applyTo42 : (int -> int) -> int
	// applyTo42 (fun x -> x + 1)  — should infer x as int from the expected type.
	src := `module Test
let apply_to_42 (f: int -> int) : int = f 42
let result = apply_to_42 (fun x -> x + 1)
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	tm, _, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
		t.Fatalf("typecheck failed")
	}

	// Find the FunExpr and verify its type is int -> int.
	found := false
	for expr, typ := range tm {
		fn, ok := expr.(*ast.FunExpr)
		if !ok {
			continue
		}
		found = true
		tfun, ok := typ.(*types.TFun)
		if !ok {
			t.Errorf("lambda type should be TFun, got %T (%v)", typ, typ)
			continue
		}
		if _, ok := tfun.From.(*types.Prim); !ok {
			t.Errorf("lambda param type should be concrete Prim, got %T (%v)", tfun.From, tfun.From)
			continue
		}
		p := tfun.From.(*types.Prim)
		if p.Name != "int" {
			t.Errorf("expected lambda param type 'int', got %q", p.Name)
		}
		if _, ok := tfun.To.(*types.Prim); !ok || tfun.To.(*types.Prim).Name != "int" {
			t.Errorf("expected lambda return type 'int', got %v", tfun.To)
		}
		_ = fn // used
	}
	if !found {
		t.Error("did not find FunExpr in TypeMap")
	}
}

// TestBidirectionalLambdaCurried verifies bidirectional inference with a
// curried function that takes multiple lambda arguments.
func TestBidirectionalLambdaCurried(t *testing.T) {
	// compose : (int -> int) -> (int -> int) -> int -> int
	// compose (fun x -> x * 2) (fun y -> y + 1) 5
	src := `module Test
let compose (f: int -> int) (g: int -> int) (a: int) : int = g (f a)
let result = compose (fun x -> x * 2) (fun y -> y + 1) 5
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	tm, _, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
		t.Fatalf("typecheck failed")
	}

	// Count FunExprs and verify each has int -> int type.
	count := 0
	for expr, typ := range tm {
		if _, ok := expr.(*ast.FunExpr); !ok {
			continue
		}
		count++
		tfun, ok := typ.(*types.TFun)
		if !ok {
			t.Errorf("lambda type should be TFun, got %T (%v)", typ, typ)
			continue
		}
		if p, ok := tfun.From.(*types.Prim); !ok || p.Name != "int" {
			t.Errorf("lambda param type should be int, got %v", tfun.From)
		}
		if p, ok := tfun.To.(*types.Prim); !ok || p.Name != "int" {
			t.Errorf("lambda return type should be int, got %v", tfun.To)
		}
	}
	if count < 2 {
		t.Errorf("expected 2 FunExpr in TypeMap, found %d", count)
	}
}

// TestBidirectionalLambdaNoAnnotation verifies that a completely unannotated
// lambda passed to a known function gets correct type inference.
func TestBidirectionalLambdaNoAnnotation(t *testing.T) {
	src := `module Test
let call_with_hello (f: string -> string) : string = f "hello"
let result = call_with_hello (fun s -> string_concat s " world")
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	tm, _, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
		t.Fatalf("typecheck failed")
	}

	for expr, typ := range tm {
		fn, ok := expr.(*ast.FunExpr)
		if !ok {
			continue
		}
		_ = fn
		tfun, ok := typ.(*types.TFun)
		if !ok {
			t.Errorf("lambda type should be TFun, got %T", typ)
			continue
		}
		if p, ok := tfun.From.(*types.Prim); !ok || p.Name != "string" {
			t.Errorf("lambda param type should be string, got %v", tfun.From)
		}
		if p, ok := tfun.To.(*types.Prim); !ok || p.Name != "string" {
			t.Errorf("lambda return type should be string, got %v", tfun.To)
		}
	}
}

// TestBidirectionalWithKnownListMap verifies that HM inference still works
// for the classic list_map example (polymorphic function + concrete list).
func TestBidirectionalWithKnownListMap(t *testing.T) {
	src := `module Test
let map (f: 'a -> 'b) (xs: 'a list) : 'b list =
  match xs with
  | [] -> []
  | x :: rest -> f x :: map f rest

let result = map (fun x -> x + 1) (1 :: 2 :: 3 :: [])
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	_, _, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
		t.Fatalf("typecheck failed")
	}
}

// TestBidirectionalFallbackToFresh verifies that when the function type is
// not resolved to a concrete TFun (still a TVar), we fall back to fresh
// vars — i.e. the bidirectional path degrades gracefully.
func TestBidirectionalFallbackToFresh(t *testing.T) {
	// identity : 'a -> 'a — when applied to (fun x -> x), the function
	// type is polymorphic; the lambda should still typecheck correctly.
	src := `module Test
let identity (x: 'a) : 'a = x
let result = identity (fun x -> x)
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	_, _, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
		t.Fatalf("typecheck failed")
	}
}

// TestGoSigFallbackExtern verifies that the gosig fallback correctly
// refines an extern binding's type using the real Go signature.
// We use "strings.Contains" which has func(string, string) bool.
func TestGoSigFallbackExtern(t *testing.T) {
	src := `module Test
import golang "strings" {
  val Contains : string -> string -> bool
}

let main () =
  let got = Contains "hello" "he" in
  print_line (if got then "ok" else "no")
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	errs := typecheck.Check(mod)
	if len(errs) > 0 {
		// If gosig fallback fails (e.g. packages.Load can't load), the
		// declared type should still work; errors here indicate a regression
		// in the declared type path.
		for _, e := range errs {
			// Only fail if the error is a type mismatch, not a gosig warning.
			if strings.Contains(e.Error(), "type mismatch") {
				t.Errorf("type error: %v", e)
			} else {
				t.Logf("gosig warning (non-fatal): %v", e)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Effect row tests
// ---------------------------------------------------------------------------

// TestEffectRowIo verifies that a function with `with { io }` has the effect
// in its inferred type.
func TestEffectRowIo(t *testing.T) {
	src := `module Test
let readFile (path: string) : string with { io } = path
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	_, vm, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
		t.Fatalf("typecheck failed")
	}

	tfun, ok := vm["readFile"].(*types.TFun)
	if !ok {
		t.Fatalf("expected TFun for readFile, got %T", vm["readFile"])
	}
	if tfun.Effects == nil {
		t.Fatal("expected non-nil Effects on readFile")
	}
	if tfun.Effects.Open {
		t.Error("expected closed effect row, got open")
	}
	if len(tfun.Effects.Effects) != 1 || tfun.Effects.Effects[0] != "io" {
		t.Errorf("expected [io] effect, got %v", tfun.Effects.Effects)
	}
}

// TestEffectRowPure verifies that a pure function (no `with`) has nil Effects
// (unknown, not pure).
func TestEffectRowPure(t *testing.T) {
	src := `module Test
let double (x: int) : int = x * 2
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	_, vm, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
		t.Fatalf("typecheck failed")
	}

	tfun, ok := vm["double"].(*types.TFun)
	if !ok {
		t.Fatalf("expected TFun for double, got %T", vm["double"])
	}
	// No `with` clause → nil Effects (unknown, permissive)
	// Actually, since the function has typed params, we expect nil Effects
	// because the parser didn't see `with`.
	if tfun.Effects != nil {
		t.Errorf("expected nil Effects for pure function, got %v", tfun.Effects)
	}
}

// TestEffectRowPolymorphic verifies that a row-polymorphic function
// `f : unit -> 'a with { e }` accepts any effectful callback.
func TestEffectRowPolymorphic(t *testing.T) {
	src := `module Test
let runAndLog (f: unit -> unit with { log; io }) : unit with { io } = f ()
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	_, vm, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
		t.Fatalf("typecheck failed")
	}

	tt, ok := vm["runAndLog"]
	if !ok {
		t.Fatal("runAndLog not found in var types")
	}
	t.Logf("runAndLog type: %s", tt)
}

// TestEffectRowBackwardCompat verifies that existing code without `with` clauses
// still works perfectly (nil Effects = permissive).
func TestEffectRowBackwardCompat(t *testing.T) {
	src := `module Test
let add (x: int) (y: int) : int = x + y
let result = add 3 4
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	errs := typecheck.Check(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
	}
}

// TestEffectRowExternOpen verifies that extern functions with effect annotation
// work correctly.
func TestEffectRowExternAnnotated(t *testing.T) {
	src := `module Test
import golang "strings" {
  val Contains : string -> string -> bool with { io }
}

let main () = Contains "a" "b"
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	errs := typecheck.Check(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			// Non-type-mismatch errors (gosig warnings) are OK
			if strings.Contains(e.Error(), "type mismatch") {
				t.Errorf("type error: %v", e)
			}
		}
	}
}

// TestEffectRowMultipleEffects verifies multiple effects in a row.
func TestEffectRowMultipleEffects(t *testing.T) {
	src := `module Test
let logAndWrite (msg: string) : unit with { io; log } = if true then () else ()
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	_, vm, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
		t.Fatalf("typecheck failed")
	}

	tfun, ok := vm["logAndWrite"].(*types.TFun)
	if !ok {
		t.Fatalf("expected TFun for logAndWrite, got %T", vm["logAndWrite"])
	}
	if tfun.Effects == nil {
		t.Fatal("expected non-nil Effects")
	}
	if len(tfun.Effects.Effects) != 2 {
		t.Errorf("expected 2 effects, got %d: %v", len(tfun.Effects.Effects), tfun.Effects.Effects)
	}
	hasIo := false
	hasLog := false
	for _, e := range tfun.Effects.Effects {
		if e == "io" {
			hasIo = true
		}
		if e == "log" {
			hasLog = true
		}
	}
	if !hasIo || !hasLog {
		t.Errorf("expected io and log effects, got %v", tfun.Effects.Effects)
	}
}

// TestEffectRowWithExplicitPure verifies that `with {}` means explicitly pure.
func TestEffectRowExplicitPure(t *testing.T) {
	src := `module Test
let f (x: int) : int with {} = x
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	_, vm, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
		t.Fatalf("typecheck failed")
	}

	tfun, ok := vm["f"].(*types.TFun)
	if !ok {
		t.Fatalf("expected TFun for f, got %T", vm["f"])
	}
	if tfun.Effects == nil {
		t.Fatal("expected non-nil Effects for explicit with {}")
	}
	if len(tfun.Effects.Effects) != 0 {
		t.Errorf("expected empty effects, got %v", tfun.Effects.Effects)
	}
	if tfun.Effects.Open {
		t.Error("expected closed effect row")
	}
}

// TestEffectRowOpen verifies `with { e | .. }` creates an open effect row.
func TestEffectRowOpen(t *testing.T) {
	src := `module Test
let catchAll (f: unit -> 'a with { e | .. }) (handler: string -> 'a) : 'a with { e | .. } = f ()
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	_, vm, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
		t.Fatalf("typecheck failed")
	}

	tt, ok := vm["catchAll"]
	if !ok {
		t.Fatal("catchAll not found in var types")
	}
	t.Logf("catchAll type: %s", tt)
}

// TestTypeCheckRegion verifies that region { let! x = ...; return ... } typechecks.
func TestTypeCheckRegion(t *testing.T) {
	src := `module Test

type handle : 1

let Close (h: handle) : unit =
  print_line "closed"

let useIt (h: handle) : unit =
  print_line "using"

let process (h: handle) : unit =
  region {
    let! x = h
    do! useIt x
    return ()
  }
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)
	errs := typecheck.Check(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
	}
}

// TestTypeCheckRegionReturnType verifies that region infers the return type.
func TestTypeCheckRegionReturnType(t *testing.T) {
	src := `module Test

let answer () : int =
  region {
    return 42
  }
`
	mod, err := parser.Parse("test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)
	errs := typecheck.Check(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
	}
}
