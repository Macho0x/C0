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
	mod := mustParse(t, "result.goop")
	gen := codegen.NewGenerator("result.goop", config.DefaultConfig())
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.Contains(goSrc, "type User struct") {
		t.Error("missing User struct")
	}
	if !strings.Contains(goSrc, "type UserError interface") {
		t.Error("missing UserError interface")
	}
	if !strings.Contains(goSrc, "func findUser") {
		t.Error("missing findUser function")
	}
	if !strings.Contains(goSrc, ".IsOk()") {
		t.Error("missing result IsOk check")
	}
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
	mod := mustParse(t, "result.goop")
	gen := codegen.NewGenerator("result.goop", config.DefaultConfig())
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "resultexample.go")
	os.WriteFile(outPath, []byte(goSrc), 0644)
	modContent := "module test\n\ngo 1.22\n"
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644)

	cmd := exec.Command("go", "build", outPath)
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
}

func TestOrderbookBuild(t *testing.T) {
	mod := mustParse(t, "orderbook.goop")
	gen := codegen.NewGenerator("orderbook.goop", config.DefaultConfig())
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Verify key signatures
	if !strings.Contains(goSrc, "func isBetter(side Side, a float64, b float64) bool") {
		t.Error("isBetter should have flattened signature func(Side, float64, float64) bool")
	}
	if !strings.Contains(goSrc, "func insertBy(greater func(float64, float64) bool, o Order, orders []Order) []Order") {
		t.Error("insertBy should have greater func(float64, float64) bool")
	}
	if !strings.Contains(goSrc, "type Side interface") {
		t.Error("missing Side interface")
	}
	if !strings.Contains(goSrc, "type SideBuy struct") {
		t.Error("missing SideBuy struct")
	}
	// Verify no interface{} in closure params (Bug 4)
	if strings.Contains(goSrc, "_p1 interface{}") || strings.Contains(goSrc, "_p2 interface{}") {
		t.Error("closure params should not use interface{}, should be float64")
	}
	// Verify no redundant rebindings (Bug 2)
	if strings.Contains(goSrc, "first := first") || strings.Contains(goSrc, "rest := rest") {
		t.Error("should not have redundant first := first rebindings")
	}
	// Verify no _c temporary vars from old CONS lowering (Bug 3)
	if strings.Contains(goSrc, "_c") {
		t.Error("should not have _c temp vars from old CONS lowering")
	}

	// Build and verify
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "orderbook.go")
	os.WriteFile(outPath, []byte(goSrc), 0644)
	modContent := "module test\n\ngo 1.22\n"
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644)

	cmd := exec.Command("go", "build", outPath)
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
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
	mod, err := parser.Parse("region_test.goop", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	gen := codegen.NewGenerator("region_test.goop", config.DefaultConfig())
	goSrc, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Verify let! emits defer Close(varName)
	if !strings.Contains(goSrc, "defer Close(x)") {
		t.Error("missing defer Close(x) for let! binding in region")
	}

	// Verify the assignment is emitted
	if !strings.Contains(goSrc, "x := h") {
		t.Error("missing x := h assignment")
	}

	// Build in temp dir to verify Go compiles
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "region.go")
	os.WriteFile(outPath, []byte(goSrc), 0644)
	modContent := "module test\n\ngo 1.22\n"
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644)

	cmdb := exec.Command("go", "build", outPath)
	cmdb.Dir = tmpDir
	if out, err := cmdb.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
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
