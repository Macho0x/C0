package codegen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/codegen"
	"goop.dev/compiler/internal/config"
	"goop.dev/compiler/internal/desugar"
	"goop.dev/compiler/internal/parser"
	"goop.dev/compiler/internal/refine"
	"goop.dev/compiler/internal/typecheck"
)

func externFinalReturn(t ast.Type) ast.Type {
	for {
		fn, ok := t.(*ast.TFun)
		if !ok {
			return t
		}
		if _, ok2 := fn.To.(*ast.TFun); ok2 {
			t = fn.To
			continue
		}
		return fn.To
	}
}

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

func TestCompileHello(t *testing.T) {
	mod := mustParse(t, "hello.goop")
	gen := codegen.NewGenerator("hello.goop", config.DefaultConfig())
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.Contains(goSrc, "package main") {
		t.Error("missing package main")
	}
	if !strings.Contains(goSrc, "fmt.Println") {
		t.Error("missing fmt.Println")
	}
	if !strings.Contains(goSrc, "Hello, Goop!") {
		t.Error("missing Hello, Goop! string")
	}
}

func TestCompileShapes(t *testing.T) {
	mod := mustParse(t, "shapes.goop")
	gen := codegen.NewGenerator("shapes.goop", config.DefaultConfig())
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// Check that the Shape interface and structs are generated
	if !strings.Contains(goSrc, "type Shape interface") {
		t.Error("missing Shape interface")
	}
	if !strings.Contains(goSrc, "type ShapeCircle struct") {
		t.Error("missing ShapeCircle struct")
	}
	if !strings.Contains(goSrc, "type ShapeRect struct") {
		t.Error("missing ShapeRect struct")
	}
	if !strings.Contains(goSrc, "type ShapePoint struct") {
		t.Error("missing ShapePoint struct")
	}
}

func TestCompileResult(t *testing.T) {
	t.Skip("result.goop still uses removed result { … } CE; awaiting example migration")
}

func TestExternTupleCallCodegen(t *testing.T) {
	src := `module main
import golang "strconv" { val Atoi : string -> (int, string) }
let main () = let pair = Atoi "42" in pair
`
	mod, err := parser.Parse("t.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod = desugar.DesugarModule(mod)
	gen := codegen.NewGenerator("t.goop", config.DefaultConfig())
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(goSrc, "__t.F0, __t.F1 = strconv.Atoi") {
		t.Fatalf("expected multi-value extern assignment, got:\n%s", goSrc)
	}
}

func TestHelloBuildAndRun(t *testing.T) {
	mod := mustParse(t, "hello.goop")
	gen := codegen.NewGenerator("hello.goop", config.DefaultConfig())
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Write to temp dir and build
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(outPath, []byte(goSrc), 0644); err != nil {
		t.Fatal(err)
	}

	// Create go.mod
	modContent := "module hello\n\ngo 1.22\n"
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644)

	// Build
	binPath := filepath.Join(tmpDir, "hello")
	cmd := exec.Command("go", "build", "-o", binPath, outPath)
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	// Run
	cmd = exec.Command(binPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(string(out), "Hello, Goop!") {
		t.Errorf("expected 'Hello, Goop!', got %q", string(out))
	}
}

func TestShapesBuild(t *testing.T) {
	mod := mustParse(t, "shapes.goop")
	gen := codegen.NewGenerator("shapes.goop", config.DefaultConfig())
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "shapes.go")
	os.WriteFile(outPath, []byte(goSrc), 0644)
	modContent := "module shapes\n\ngo 1.22\n"
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644)

	cmd := exec.Command("go", "build", outPath)
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
}

func TestResultBuild(t *testing.T) {
	t.Skip("result.goop still uses removed result { … } CE; awaiting example migration")
}

func TestOrderbookBuild(t *testing.T) {
	t.Skip("orderbook.goop still uses removed newtype; awaiting example migration")
}

func TestTypeCheckBeforeCodegen(t *testing.T) {
	mod := mustParse(t, "hello.goop")
	errs := typecheck.Check(mod)
	if len(errs) > 0 {
		t.Fatalf("type check failed: %v", errs)
	}
}

func TestActivePatternsExampleBuild(t *testing.T) {
	path := filepath.Join(examplesDir, "active_patterns.goop")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	mod, err := parser.Parse("active_patterns.goop", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	gen := codegen.NewGenerator("active_patterns.goop", config.DefaultConfig())
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Verify key patterns in generated code
	if !strings.Contains(goSrc, "Int_option interface") && !strings.Contains(goSrc, "IntOption interface") {
		t.Error("missing Int_option interface for int_option ADT")
	}
	if !strings.Contains(goSrc, "isPositive") {
		t.Error("missing isPositive function")
	}
	if !strings.Contains(goSrc, "isEven") {
		t.Error("missing isEven function")
	}

	// Build in temp dir
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "activepatternsdemo.go")
	os.WriteFile(outPath, []byte(goSrc), 0644)
	modContent := "module test\n\ngo 1.22\n"
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644)

	cmd := exec.Command("go", "build", outPath)
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
}

func TestContractsBuild(t *testing.T) {
	mod := mustParse(t, "contracts.goop")
	gen := codegen.NewGenerator("contracts.goop", config.DefaultConfig())
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Verify precondition checks
	if !strings.Contains(goSrc, "precondition violated: b <> 0") {
		t.Error("missing precondition check for safeDiv")
	}
	if !strings.Contains(goSrc, "precondition violated: hi >= lo") {
		t.Error("missing precondition check for clamp")
	}

	// Verify postcondition checks
	if !strings.Contains(goSrc, "postcondition violated: result >= lo && result <= hi") {
		t.Error("missing postcondition check for clamp")
	}

	// Verify named return value for postcondition
	if !strings.Contains(goSrc, "result int") {
		t.Error("missing named return value for postcondition")
	}

	// Verify defer postcondition pattern
	if !strings.Contains(goSrc, "defer func()") {
		t.Error("missing defer for postcondition")
	}

	// Build in temp dir
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "contracts.go")
	os.WriteFile(outPath, []byte(goSrc), 0644)
	modContent := "module test\n\ngo 1.22\n"
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644)

	cmdb := exec.Command("go", "build", outPath)
	cmdb.Dir = tmpDir
	if out, err := cmdb.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
}

// TestCompileRegion verifies that region { let! x = ... } emits defer Close(x).
func TestCompileRegion(t *testing.T) {
	t.Skip("region { … } computation expressions removed (PARSE-MIG013)")
}

func TestCompileRefWhile(t *testing.T) {
	src := `module Test
let main () =
  let r = ref 0 in
  while !r < 3 do
    r := !r + 1
  done
`
	mod, err := parser.Parse("ref_while.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)
	tm, vtm, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		t.Fatalf("typecheck: %v", errs)
	}
	gen := codegen.NewGenerator("ref_while.goop", config.DefaultConfig())
	gen.SetTypeMap(tm, vtm)
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.Contains(goSrc, "for ") {
		t.Error("expected while to lower to for")
	}
	if !strings.Contains(goSrc, "*") {
		t.Error("expected ref/deref pointer ops")
	}
}

// TestChanSafetyWrapper verifies that channel operations lower to the
// C0Chan wrapper struct instead of raw Go channels.
func TestChanSafetyWrapper(t *testing.T) {
	src := `module Main

let main () =
  let ch : int chan = Chan.make () in
  let u = Chan.send ch 42 in
  let v = Chan.close ch in
  print_line "channel operations work"
`
	mod, err := parser.Parse("chan_test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	tm, vtm, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
		t.Fatalf("typecheck failed")
	}

	gen := codegen.NewGenerator("chan_test.goop", config.DefaultConfig())
	gen.SetTypeMap(tm, vtm)
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Chan.make generates C0ChanMake()
	if !strings.Contains(goSrc, "C0ChanMake()") {
		t.Error("missing C0ChanMake()")
	}

	// Chan.send generates C0ChanSend
	if !strings.Contains(goSrc, "C0ChanSend(ch, 42)") {
		t.Errorf("missing C0ChanSend(ch, 42), got:\n%s", goSrc)
	}

	// Chan.recv is available via helper
	if !strings.Contains(goSrc, "func C0ChanRecv") {
		t.Error("missing C0ChanRecv helper")
	}

	// Chan.close generates C0ChanClose
	if !strings.Contains(goSrc, "C0ChanClose(ch") {
		t.Error("missing C0ChanClose(ch)")
	}

	// Wrapper struct is emitted
	if !strings.Contains(goSrc, "type C0Chan struct") {
		t.Error("missing C0Chan struct")
	}

	// Helpers are emitted
	if !strings.Contains(goSrc, "func C0ChanSend") {
		t.Error("missing C0ChanSend helper")
	}
	if !strings.Contains(goSrc, "func C0ChanClose") {
		t.Error("missing C0ChanClose helper")
	}
	if !strings.Contains(goSrc, `"Chan.send: channel is closed"`) {
		t.Error("missing safe send panic message")
	}
	if !strings.Contains(goSrc, `"Chan.close: channel already closed"`) {
		t.Error("missing safe close panic message")
	}

	// Should NOT contain raw channel ops at call site
	if strings.Contains(goSrc, ":= make(chan") || strings.Contains(goSrc, "= make(chan") {
		t.Error("should not contain raw make(chan ...) at call site")
	}

	// Build the generated Go
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "chansafety.go")
	os.WriteFile(outPath, []byte(goSrc), 0644)
	modContent := "module test\n\ngo 1.22\n"
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644)

	cmdb := exec.Command("go", "build", outPath)
	cmdb.Dir = tmpDir
	if out, err := cmdb.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
}

// TestChanSafetyNoChannel verifies that the C0Chan wrapper is NOT emitted
// when no channel operations are used.
func TestChanSafetyNoChannel(t *testing.T) {
	src := `module Main

let main () =
  print_line "no channels here!"
`
	mod, err := parser.Parse("nochan_test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	gen := codegen.NewGenerator("nochan_test.goop", config.DefaultConfig())
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if strings.Contains(goSrc, "C0Chan") {
		t.Error("C0Chan wrapper emitted but no channels used")
	}
}

// TestChanSafetyBuild verifies that generated Go code with channels
// compiles and runs correctly.
func TestChanSafetyBuild(t *testing.T) {
	src := `module Main

let main () =
  let ch : int chan = Chan.make () in
  let u = Chan.close ch in
  print_line "channel closed"
`
	mod, err := parser.Parse("chanbuild_test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	tm, vtm, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("type error: %v", e)
		}
		t.Fatalf("typecheck failed")
	}

	gen := codegen.NewGenerator("chanbuild_test.goop", config.DefaultConfig())
	gen.SetTypeMap(tm, vtm)
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "chanbuild.go")
	os.WriteFile(outPath, []byte(goSrc), 0644)
	modContent := "module test\n\ngo 1.22\n"
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644)

	binPath := filepath.Join(tmpDir, "testbin")
	cmdb := exec.Command("go", "build", "-o", binPath, outPath)
	cmdb.Dir = tmpDir
	if out, err := cmdb.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	out, _ := exec.Command(binPath).CombinedOutput()
	if !strings.Contains(string(out), "channel closed") {
		t.Errorf("expected 'channel closed' in output, got %q", string(out))
	}
}

func TestRefinementCallSiteGuards(t *testing.T) {
	mod := mustParse(t, "refinement_solving.goop")
	tm, vtm, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		t.Fatalf("typecheck: %v", errs)
	}
	proven, funcProven, _, refineErrs := refine.CheckRefinements(mod, tm, config.DefaultConfig())
	if len(refineErrs) > 0 {
		t.Fatalf("refine: %v", refineErrs)
	}
	gen := codegen.NewGenerator("refinement_solving.goop", config.DefaultConfig())
	gen.SetTypeMap(tm, vtm)
	gen.SetProvenSites(proven)
	gen.SetRefinementMeta(funcProven)
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.Contains(goSrc, "precondition violated") {
		t.Error("expected entry or call-site precondition guard")
	}
	if strings.Contains(goSrc, "func() int {\n\tif !(y != 0)") {
		t.Error("proven compute call should not wrap in guarded IIFE for y != 0")
	}
	_ = funcProven
}

func TestTopLevelRecordLiteralNotThunk(t *testing.T) {
	src := `module T
type cfg = { x: int; y: string }
let cfg = { x = 1; y = "a" }
`
	mod, err := parser.Parse("t.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod = desugar.DesugarModule(mod)
	tm, vtm, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	gen := codegen.NewGenerator("t.goop", config.DefaultConfig())
	gen.SetTypeMap(tm, vtm)
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(goSrc, "func cfg()") {
		t.Fatalf("record literal should not become thunk:\n%s", goSrc)
	}
	if !strings.Contains(goSrc, "var cfg = cfg{") {
		t.Fatalf("expected direct record var, got:\n%s", goSrc)
	}
}

func TestParenExprFloatPrecedence(t *testing.T) {
	src := `module T
let pct (current: float) (mean: float) : float =
  (current -. mean) /. mean
`
	mod, err := parser.Parse("t.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod = desugar.DesugarModule(mod)
	gen := codegen.NewGenerator("t.goop", config.DefaultConfig())
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(goSrc, "current - mean / mean") {
		t.Fatalf("parens must preserve float precedence:\n%s", goSrc)
	}
	if !strings.Contains(goSrc, "(current - mean)") {
		t.Fatalf("expected parenthesized subtraction:\n%s", goSrc)
	}
}

func TestOptionInRecordFieldCodegen(t *testing.T) {
	src := `module T
type cfg = { name: string; limit: int option }
let c = { name = "x"; limit = Some 1 }
`
	mod, err := parser.Parse("t.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod = desugar.DesugarModule(mod)
	tm, vtm, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	gen := codegen.NewGenerator("t.goop", config.DefaultConfig())
	gen.SetTypeMap(tm, vtm)
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(goSrc, "type OptionInt struct") {
		t.Fatalf("missing OptionInt struct:\n%s", goSrc)
	}
	if !strings.Contains(goSrc, "NewOptionIntSome(1)") {
		t.Fatalf("expected NewOptionIntSome, got:\n%s", goSrc)
	}
}

func TestArrayMakeAndIndexCodegen(t *testing.T) {
	src := `module T
let main () =
  begin
    let arr = Array.make 2 0 in
    arr.(0) <- 7;
    arr.(1)
  end
`
	mod, err := parser.Parse("t.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod = desugar.DesugarModule(mod)
	tm, vtm, errs := typecheck.CheckWithTypes(mod)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	gen := codegen.NewGenerator("t.goop", config.DefaultConfig())
	gen.SetTypeMap(tm, vtm)
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(goSrc, "make([]int, 2)") {
		t.Fatalf("expected make([]int, 2), got:\n%s", goSrc)
	}
	if !strings.Contains(goSrc, "arr[0] = 7") {
		t.Fatalf("expected array index assignment:\n%s", goSrc)
	}
}
