// Command c0 is the C0 language compiler CLI.
//
// Usage:
//
//	c0 lex  <file.c0>    lexical analysis — print token stream
//	c0 parse <file.c0>    parse — pretty-print AST
//	c0 check <file.c0>    parse and report success/failure
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"c0.dev/compiler/internal/ast"
	"c0.dev/compiler/internal/codegen"
	"c0.dev/compiler/internal/color"
	"c0.dev/compiler/internal/config"
	"c0.dev/compiler/internal/desugar"
	"c0.dev/compiler/internal/exhaustive"
	lc0 "c0.dev/compiler/internal/lexer"
	"c0.dev/compiler/internal/linear"
	"c0.dev/compiler/internal/parser"
	"c0.dev/compiler/internal/refine"
	"c0.dev/compiler/internal/report"
	"c0.dev/compiler/internal/token"
	"c0.dev/compiler/internal/typecheck"
	"c0.dev/compiler/internal/typeinfo"
)

// LSP types for protocol messages
type Request struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	Jsonrpc string         `json:"jsonrpc"`
	ID      interface{}    `json:"id,omitempty"`
	Result  interface{}    `json:"result,omitempty"`
	Error   *ResponseError `json:"error,omitempty"`
}

type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Message  string `json:"message"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// LSP error severity constants (1=error, 2=warning, 3=info, 4=hint)
const (
	SeverityError   = 1
	SeverityWarning = 2
	SeverityInfo    = 3
	SeverityHint    = 4
)

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

type ServerCapabilities struct {
	TextDocumentSync   int  `json:"textDocumentSync"`
	HoverProvider      bool `json:"hoverProvider"`
	DefinitionProvider bool `json:"definitionProvider"`
	CompletionProvider bool `json:"completionProvider"`
}

func main() {
	// Parse optional flags
	args := os.Args[1:]
	globalWriteMap := true
	colorOutput := false
	inPlace := false // -i flag for in-place formatting
	var filtered []string
	for _, a := range args {
		if a == "--no-source-map" {
			globalWriteMap = false
		} else if a == "--color" {
			colorOutput = true
		} else if a == "-i" || a == "--in-place" {
			inPlace = true
		} else {
			filtered = append(filtered, a)
		}
	}

	if len(filtered) < 2 && (len(filtered) == 0 || filtered[0] != "test") && filtered[0] != "lsp" && filtered[0] != "fmt" {
		fmt.Fprintf(os.Stderr, "Usage: c0 [--no-source-map] [--color] [-i] <command> <file.c0>\n")
		fmt.Fprintf(os.Stderr, "Commands: lex, parse, check, compile, build, test, resolve, lsp, fmt (format)\n")
		os.Exit(1)
	}

	cmd := filtered[0]

	// `c0 test` and `c0 lsp` are special: they don't need a file argument
	if cmd == "test" {
		dir := "."
		if len(filtered) >= 2 {
			dir = filtered[1]
		}
		os.Exit(runTests(dir))
	}
	if cmd == "lsp" {
		runLSP()
		return
	}

	file := filtered[1]

	// Load project config (look for c0.toml near the source file or CWD)
	cfg := loadProjectConfig(file)

	src, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", file, err)
		os.Exit(1)
	}

	// Helper to parse and desugar
	parseAndDesugar := func() (*ast.Module, error) {
		mod, err := parser.Parse(file, src)
		if err != nil {
			return nil, err
		}
		return desugar.DesugarModule(mod), nil
	}

	switch cmd {
	case "lex":
		toks, err := lc0.Lex(file, src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "lex error: %v\n", err)
			os.Exit(1)
		}
		// Filter out EOF for cleaner output
		if colorOutput {
			for _, t := range toks {
				if t.Type.String() == "EOF" {
					continue
				}
				colored := color.TokenColor(t.Type) + t.String() + color.Reset
				fmt.Printf("%4d:%-3d  %s\n", t.Loc.Line, t.Loc.Column, colored)
			}
		} else {
			for _, t := range toks {
				if t.Type.String() == "EOF" {
					continue
				}
				fmt.Printf("%4d:%-3d  %s\n", t.Loc.Line, t.Loc.Column, t.String())
			}
		}

	case "parse":
		mod, err := parseAndDesugar()
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(printModule(mod))

	case "check":
		mod, err := parseAndDesugar()
		if err != nil {
			fmt.Printf("FAIL: parse error: %v\n", err)
			os.Exit(1)
		}

		// Register ADTs from type declarations for exhaustiveness checking
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

		// Type checking
		typeErrors := typecheck.Check(mod)
		if len(typeErrors) > 0 {
			fmt.Println("FAIL: type errors:")
			for _, e := range typeErrors {
				fmt.Print(report.Render(e, src))
			}
			os.Exit(1)
		}

		// Linear discharge checking
		linearTypes := buildLinearTypes(mod)
		linearErrors := linear.Check(mod, linearTypes)
		if len(linearErrors) > 0 {
			fmt.Println("FAIL: linear discharge errors:")
			for _, e := range linearErrors {
				fmt.Print(report.Render(e, src))
			}
			os.Exit(1)
		}

		// Exhaustiveness checking
		exhaustWarnings := exhaustive.Check(mod)
		if len(exhaustWarnings) > 0 {
			fmt.Println("WARN: exhaustiveness warnings:")
			for _, w := range exhaustWarnings {
				fmt.Print(report.Render(w, src))
			}
		}

		fmt.Printf("OK: %s parsed and type-checked successfully\n", file)

	case "compile":
		mod, err := parseAndDesugar()
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
			os.Exit(1)
		}

		// Type-check (for validation and to get inferred types for codegen)
		tm, vtm, typeErrs := typecheck.CheckWithTypes(mod)
		if len(typeErrs) > 0 {
			fmt.Println("FAIL: type errors:")
			for _, e := range typeErrs {
				fmt.Print(report.Render(e, src))
			}
			os.Exit(1)
		}

		// Linear discharge checking
		linearTypes := buildLinearTypes(mod)
		if linearErrs := linear.Check(mod, linearTypes); len(linearErrs) > 0 {
			fmt.Println("FAIL: linear discharge errors:")
			for _, e := range linearErrs {
				fmt.Print(report.Render(e, src))
			}
			os.Exit(1)
		}

		// Refinement contract checking (compile-time VC generation)
		proven, refineWarnings, refineErrs := refine.CheckRefinements(mod, tm)
		for _, w := range refineWarnings {
			fmt.Fprintf(os.Stderr, "WARNING: %v\n", w)
		}
		if len(refineErrs) > 0 {
			fmt.Println("FAIL: refinement errors:")
			for _, e := range refineErrs {
				fmt.Print(report.Render(e, src))
			}
			os.Exit(1)
		}

		// Generate Go code
		gen := codegen.NewGenerator(file, cfg)
		gen.SetTypeMap(tm, vtm)
		gen.SetProvenSites(proven)
		goSrc, err := gen.Generate(mod)
		if err != nil {
			fmt.Fprintf(os.Stderr, "codegen error: %v\n", err)
			os.Exit(1)
		}

		// Determine output file name
		outFile := gen.GoFileName()
		dir := filepath.Dir(file)
		outPath := filepath.Join(dir, outFile)

		if err := os.WriteFile(outPath, []byte(goSrc), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s\n", outPath)

		// Write source map (unless --no-source-map flag is present)
		if globalWriteMap {
			sm := gen.SourceMap()
			if sm != nil {
				mapPath := outPath + ".map.json"
				f, err := os.Create(mapPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "source map create error: %v\n", err)
				} else {
					if err := sm.Write(f); err != nil {
						fmt.Fprintf(os.Stderr, "source map write error: %v\n", err)
					}
					f.Close()
					fmt.Printf("wrote %s\n", mapPath)
				}
			}
		}

	case "build":
		mod, err := parseAndDesugar()
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
			os.Exit(1)
		}

		// Type-check
		tm, vtm, typeErrs := typecheck.CheckWithTypes(mod)
		if len(typeErrs) > 0 {
			fmt.Println("FAIL: type errors:")
			for _, e := range typeErrs {
				fmt.Print(report.Render(e, src))
			}
			os.Exit(1)
		}

		// Linear discharge checking
		linearTypes := buildLinearTypes(mod)
		if linearErrs := linear.Check(mod, linearTypes); len(linearErrs) > 0 {
			fmt.Println("FAIL: linear discharge errors:")
			for _, e := range linearErrs {
				fmt.Print(report.Render(e, src))
			}
			os.Exit(1)
		}

		// Refinement contract checking (compile-time VC generation)
		proven, refineWarnings, refineErrs := refine.CheckRefinements(mod, tm)
		for _, w := range refineWarnings {
			fmt.Fprintf(os.Stderr, "WARNING: %v\n", w)
		}
		if len(refineErrs) > 0 {
			fmt.Println("FAIL: refinement errors:")
			for _, e := range refineErrs {
				fmt.Print(report.Render(e, src))
			}
			os.Exit(1)
		}

		// Generate Go code
		gen := codegen.NewGenerator(file, cfg)
		gen.SetTypeMap(tm, vtm)
		gen.SetProvenSites(proven)
		goSrc, err := gen.Generate(mod)
		if err != nil {
			fmt.Fprintf(os.Stderr, "codegen error: %v\n", err)
			os.Exit(1)
		}

		// Determine build directory and package name
		srcDir := filepath.Dir(file)
		goModPath := filepath.Join(srcDir, "go.mod")
		hasGoMod := false
		if _, err := os.Stat(goModPath); err == nil {
			hasGoMod = true
		}

		// Collect any existing .go files (excluding generated one)
		goFiles, _ := filepath.Glob(filepath.Join(srcDir, "*.go"))
		genFile := gen.GoFileName()
		isMixed := len(goFiles) > 0

		// Write generated file next to sources
		outPath := filepath.Join(srcDir, genFile)
		if err := os.WriteFile(outPath, []byte(goSrc), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s\n", outPath)

		// Build in source directory
		buildDir := srcDir
		if !hasGoMod {
			// Create minimal go.mod for mixed build
			modContent := "module c0build\n\ngo 1.22\n"
			os.WriteFile(goModPath, []byte(modContent), 0644)
			fmt.Printf("created %s (temporary)\n", goModPath)
		}

		var cmd *exec.Cmd
		if isMixed || hasGoMod {
			// Build the whole package (includes hand-written .go files)
			cmd = exec.Command("go", "build", ".")
		} else {
			cmd = exec.Command("go", "build", "-o", "c0-out", genFile)
		}
		cmd.Dir = buildDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "go build failed:\n%s\n", output)
			os.Exit(1)
		}
		fmt.Printf("build succeeded in %s\n", buildDir)

	case "resolve":
		mod, err := parseAndDesugar()
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
			os.Exit(1)
		}
		if len(mod.Opens) == 0 {
			fmt.Println("(no open statements)")
		}
		for _, o := range mod.Opens {
			goPath, goPkg := cfg.ResolveImport(o.Path)
			fmt.Printf("open %-20s → %-35s (package %s)\n", o.Path, goPath, goPkg)
		}

	case "fmt":
		mod, err := parseAndDesugar()
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
			os.Exit(1)
		}
		formatted := FormatModule(mod)
		if inPlace {
			outPath := file
			if err := os.WriteFile(outPath, []byte(formatted), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "write error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("formatted %s\n", outPath)
		} else {
			fmt.Print(formatted)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		fmt.Fprintf(os.Stderr, "Commands: lex, parse, check, compile, build, test, resolve, lsp, fmt\n")
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// LSP Server implementation
// ---------------------------------------------------------------------------

func runLSP() {
	server := &LSPServer{
		documents: make(map[string]string),
		states:    make(map[string]*DocumentState),
	}
	stdin := bufio.NewReader(os.Stdin)
	stdout := os.Stdout

	for {
		req, err := readLSPMessage(stdin)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error reading message: %v", err)
			continue
		}
		if req.Method == "" {
			continue
		}
		resp := server.handleLSPRequest(req)
		sendLSPResponse(stdout, resp)
	}
}

type LSPServer struct {
	documents map[string]string
	states    map[string]*DocumentState // document states by URI
}

// DocumentState holds parsed module and type info for a document
type DocumentState struct {
	Module  *ast.Module
	TypeMap typeinfo.TypeMap
	VarMap  typeinfo.VarTypeMap
	Errors  []error
	Src     []byte
}

func (s *LSPServer) handleLSPRequest(req Request) Response {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "textDocument/didOpen":
		return s.handleDidOpen(req)
	case "textDocument/didChange":
		return s.handleDidChange(req)
	case "textDocument/didSave":
		return s.handleDidSave(req)
	case "textDocument/hover":
		return s.handleHover(req)
	case "textDocument/definition":
		return s.handleDefinition(req)
	case "textDocument/completion":
		return s.handleCompletion(req)
	default:
		return Response{
			Jsonrpc: "2.0",
			ID:      req.ID,
			Error: &ResponseError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

func (s *LSPServer) handleInitialize(req Request) Response {
	return Response{
		Jsonrpc: "2.0",
		ID:      req.ID,
		Result: InitializeResult{
			Capabilities: ServerCapabilities{
				TextDocumentSync:   1,
				HoverProvider:      true,
				DefinitionProvider: true,
				CompletionProvider: true,
			},
		},
	}
}

func (s *LSPServer) handleDidSave(req Request) Response {
	s.handleDocumentUpdate(req.Params)
	return Response{Jsonrpc: "2.0", ID: req.ID}
}

func (s *LSPServer) handleDidOpen(req Request) Response {
	s.handleDocumentUpdate(req.Params)
	return Response{Jsonrpc: "2.0", ID: req.ID}
}

func (s *LSPServer) handleDidChange(req Request) Response {
	s.handleDocumentUpdate(req.Params)
	return Response{Jsonrpc: "2.0", ID: req.ID}
}

// handleDocumentUpdate parses and type-checks a document, storing results for LSP use.
func (s *LSPServer) handleDocumentUpdate(params json.RawMessage) {
	var p struct {
		TextDocument struct {
			URI  string `json:"uri"`
			Text string `json:"text"`
		} `json:"textDocument"`
	}
	json.Unmarshal(params, &p)

	uri := p.TextDocument.URI
	src := []byte(p.TextDocument.Text)
	fileName := uriToPath(uri)

	// Store source text
	s.documents[uri] = p.TextDocument.Text

	mod, parseErr := parser.Parse(fileName, src)
	var diagnostics []Diagnostic

	if parseErr != nil {
		diagnostics = append(diagnostics, diagnosticFromError(parseErr, src))
	} else {
		mod = desugar.DesugarModule(mod)

		// Register ADTs for exhaustiveness checking
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

		tm, vtm, typeErrs := typecheck.CheckWithTypes(mod)
		for _, terr := range typeErrs {
			diagnostics = append(diagnostics, diagnosticFromError(terr, src))
		}

		// Linear discharge checking
		linearTypes := buildLinearTypes(mod)
		linearErrors := linear.Check(mod, linearTypes)
		for _, terr := range linearErrors {
			diagnostics = append(diagnostics, diagnosticFromError(terr, src))
		}

		// Store state for hover/definition
		s.states[uri] = &DocumentState{
			Module:  mod,
			TypeMap: tm,
			VarMap:  vtm,
			Src:     src,
		}
	}

	s.publishDiagnostics(uri, diagnostics)
}

// diagnosticFromError creates a Diagnostic with proper LSP range from an error.
func diagnosticFromError(err error, src []byte) Diagnostic {
	var diag Diagnostic
	diag.Severity = SeverityError

	// Try to extract SourceLoc from known error types
	var loc token.SourceLoc

	// Check for TypeError (has Loc field)
	if te, ok := err.(*typecheck.TypeError); ok {
		loc = te.Loc
		diag.Message = te.Msg
	} else if msg, ok := err.(interface{ GetLoc() token.SourceLoc }); ok {
		// For nilchan.Error and similar types that might have GetLoc method
		loc = msg.GetLoc()
		diag.Message = err.Error()
	} else {
		// Parse location from error message: "file:line:col: msg"
		emsg := err.Error()
		parts := strings.SplitN(emsg, ":", 4)
		if len(parts) >= 4 {
			loc.Line, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
			loc.Column, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
			diag.Message = strings.TrimSpace(parts[3])
		} else {
			diag.Message = emsg
		}
	}

	// Convert to LSP range (0-based line, 0-based character)
	diag.Range = lspRangeFromLoc(loc)
	return diag
}

// getExprLoc extracts the source location from an expression node.
func getExprLoc(e ast.Expr) token.SourceLoc {
	switch e := e.(type) {
	case *ast.LitExpr:
		return e.Loc
	case *ast.IdentExpr:
		return e.Loc
	case *ast.ConstructorExpr:
		return e.Loc
	case *ast.AppExpr:
		return e.Loc
	case *ast.IfExpr:
		return e.Loc
	case *ast.MatchExpr:
		return e.Loc
	case *ast.LetInExpr:
		return e.Loc
	case *ast.FunExpr:
		return e.Loc
	case *ast.GuardExpr:
		return e.Loc
	case *ast.IsExpr:
		return e.Loc
	case *ast.AsMatchExpr:
		return e.Loc
	case *ast.BinaryExpr:
		return e.Loc
	case *ast.PipeExpr:
		return e.Loc
	case *ast.QuestionExpr:
		return e.Loc
	case *ast.RecordExpr:
		return e.Loc
	case *ast.RecordUpdateExpr:
		return e.Loc
	case *ast.FieldAccessExpr:
		return e.Loc
	case *ast.TupleExpr:
		return e.Loc
	case *ast.ListExpr:
		return e.Loc
	case *ast.ParenExpr:
		return e.Loc
	case *ast.GoExpr:
		return e.Loc
	case *ast.SelectExpr:
		return e.Loc
	case *ast.UsingExpr:
		return e.Loc
	case *ast.RegionExpr:
		return e.Loc
	case *ast.CompExpr:
		return e.Loc
	default:
		return token.SourceLoc{}
	}
}

// lspRangeFromLoc converts a SourceLoc to an LSP Range.
// LSP uses 0-based line and character positions.
func lspRangeFromLoc(loc token.SourceLoc) Range {
	// For now, point diagnostics at the start of the line
	// More precise would be to find the token span
	return Range{
		Start: Position{
			Line:      max(0, loc.Line-1),
			Character: max(0, loc.Column-1),
		},
		End: Position{
			Line:      max(0, loc.Line-1),
			Character: max(0, loc.Column-1),
		},
	}
}

// Hover response types
type HoverResponse struct {
	Contents HoverContents `json:"contents"`
}

type HoverContents struct {
	Kind  string `json:"kind,omitempty"`
	Value string `json:"value,omitempty"`
}

func (s *LSPServer) handleHover(req Request) Response {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
	}
	json.Unmarshal(req.Params, &params)

	state, ok := s.states[params.TextDocument.URI]
	if !ok {
		return Response{Jsonrpc: "2.0", ID: req.ID, Result: nil}
	}

	// Find the expression at this position
	hoverText, found := s.findHoverInfo(state, params.Position)
	if !found {
		return Response{Jsonrpc: "2.0", ID: req.ID, Result: nil}
	}

	return Response{
		Jsonrpc: "2.0",
		ID:      req.ID,
		Result: HoverResponse{
			Contents: HoverContents{
				Kind:  "plaintext",
				Value: hoverText,
			},
		},
	}
}

func (s *LSPServer) findHoverInfo(state *DocumentState, pos struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}) (string, bool) {
	if state.Module == nil {
		return "", false
	}

	// Use 1-based line/column for comparison
	loc := token.SourceLoc{
		Line: pos.Line + 1,
	}

	// Find the expression at this position by checking all expressions in TypeMap
	// We look for the expression whose start location is closest to the cursor
	var bestExpr ast.Expr
	var bestCol int

	for expr := range state.TypeMap {
		eloc := getExprLoc(expr)
		if eloc.Line == loc.Line && eloc.Column <= pos.Character+1 {
			// Check if this expression is closer to cursor than our best so far
			if bestExpr == nil || eloc.Column > bestCol {
				bestExpr = expr
				bestCol = eloc.Column
			}
		}
	}

	if bestExpr != nil {
		if typ, ok := state.TypeMap[bestExpr]; ok {
			return typ.String(), true
		}
	}

	return "", false
}

// Location response types
type LocationResponse struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

func (s *LSPServer) handleDefinition(req Request) Response {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
	}
	json.Unmarshal(req.Params, &params)

	state, ok := s.states[params.TextDocument.URI]
	if !ok || state.Module == nil {
		return Response{Jsonrpc: "2.0", ID: req.ID, Result: nil}
	}

	loc := s.findDefinition(state, params.Position)
	if loc == nil {
		return Response{Jsonrpc: "2.0", ID: req.ID, Result: nil}
	}

	loc.URI = params.TextDocument.URI
	return Response{
		Jsonrpc: "2.0",
		ID:      req.ID,
		Result:  loc,
	}
}

func (s *LSPServer) findDefinition(state *DocumentState, pos struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}) *LocationResponse {
	// Look for identifier references that refer to a let binding
	loc := token.SourceLoc{
		Line:   pos.Line + 1,
		Column: pos.Character + 1,
	}

	// Find identifier at this location by checking all expressions in TypeMap
	for expr := range state.TypeMap {
		ident, ok := expr.(*ast.IdentExpr)
		if !ok {
			continue
		}
		if ident.Loc.Line == loc.Line && ident.Loc.Column == loc.Column {
			// Found the identifier - look for its definition
			for _, d := range state.Module.Decls {
				ld, ok := d.(*ast.LetDecl)
				if !ok {
					continue
				}
				for _, b := range ld.Bindings {
					if b.Name == ident.Name && b.Body != nil {
						// Found definition - return its location
						return &LocationResponse{
							URI:   "", // Will be filled by caller
							Range: lspRangeFromLoc(getExprLoc(b.Body)),
						}
					}
				}
			}
			break
		}
	}
	return nil
}

// Completion response types
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

type CompletionItem struct {
	Label         string `json:"label"`
	Kind          int    `json:"kind,omitempty"`
	Detail        string `json:"detail,omitempty"`
	Documentation string `json:"documentation,omitempty"`
}

func (s *LSPServer) handleCompletion(req Request) Response {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
	}
	json.Unmarshal(req.Params, &params)

	state, ok := s.states[params.TextDocument.URI]
	if !ok || state.Module == nil {
		return Response{Jsonrpc: "2.0", ID: req.ID, Result: nil}
	}

	items := s.collectCompletions(state)
	return Response{
		Jsonrpc: "2.0",
		ID:      req.ID,
		Result: CompletionList{
			IsIncomplete: false,
			Items:        items,
		},
	}
}

func (s *LSPServer) collectCompletions(state *DocumentState) []CompletionItem {
	var items []CompletionItem

	// Collect from type declarations
	for _, d := range state.Module.Decls {
		switch d := d.(type) {
		case *ast.TypeDecl:
			items = append(items, CompletionItem{
				Label: d.Name,
				Kind:  22, // Class (for types)
			})
			if adt, ok := d.Kind.(*ast.ADTTypeKind); ok {
				for _, c := range adt.Cases {
					items = append(items, CompletionItem{
						Label: c.Name,
						Kind:  11, // EnumMember (for constructors)
					})
				}
			}
		case *ast.LetDecl:
			for _, b := range d.Bindings {
				items = append(items, CompletionItem{
					Label: b.Name,
					Kind:  12, // Function
				})
			}
		}
	}

	return items
}

func (s *LSPServer) publishDiagnostics(uri string, diagnostics []Diagnostic) {
	notif := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "textDocument/publishDiagnostics",
		"params": map[string]interface{}{
			"uri":         uri,
			"diagnostics": diagnostics,
		},
	}
	enc, _ := json.Marshal(notif)
	fmt.Printf("Content-Length: %d\r\n\r\n%s", len(enc), enc)
}

func readLSPMessage(r io.Reader) (Request, error) {
	var req Request
	scanner := bufio.NewScanner(r)
	var contentLength int
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			fmt.Sscanf(line, "Content-Length: %d", &contentLength)
		}
	}
	if contentLength > 0 {
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(r, body); err != nil {
			return req, err
		}
		json.Unmarshal(body, &req)
	}
	return req, nil
}

func sendLSPResponse(w io.Writer, resp Response) {
	enc, _ := json.Marshal(resp)
	fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(enc), enc)
}

// Convert file URI to path
func uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return uri[7:]
	}
	return uri
}

// ---------------------------------------------------------------------------
// Project configuration
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Test runner
// ---------------------------------------------------------------------------

const c0ProjectImportPrefix = "github.com/Macho0x/C0/"

func findProjectRoot(start string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "c0.toml")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "std")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func localC0PathForImport(projectRoot, goImport string) string {
	if !strings.HasPrefix(goImport, c0ProjectImportPrefix) {
		return ""
	}
	rel := strings.TrimPrefix(goImport, c0ProjectImportPrefix)
	pkg := filepath.Base(rel)
	return filepath.Join(projectRoot, filepath.FromSlash(rel), pkg+".c0")
}

func compileC0ModuleToGo(c0Path string, cfg *config.Config) (string, string, error) {
	src, err := os.ReadFile(c0Path)
	if err != nil {
		return "", "", err
	}
	mod, err := parser.Parse(c0Path, src)
	if err != nil {
		return "", "", err
	}
	mod = desugar.DesugarModule(mod)
	tm, vtm, typeErrs := typecheck.CheckWithTypes(mod)
	if len(typeErrs) > 0 {
		return "", "", fmt.Errorf("type errors in %s: %v", c0Path, typeErrs[0])
	}
	gen := codegen.NewGenerator(c0Path, cfg)
	gen.SetTypeMap(tm, vtm)
	goSrc, err := gen.Generate(mod)
	if err != nil {
		return "", "", err
	}
	return goSrc, gen.GoFileName(), nil
}

func writeOpenDependencies(mod *ast.Module, cfg *config.Config, projectRoot, tmpDir string) (string, error) {
	if projectRoot == "" || len(mod.Opens) == 0 {
		return "module test\n\ngo 1.22\n", nil
	}
	var replaces []string
	var requires []string
	compiled := make(map[string]bool)
	for _, o := range mod.Opens {
		goPath, _ := cfg.ResolveImport(o.Path)
		c0Path := localC0PathForImport(projectRoot, goPath)
		if c0Path == "" {
			continue
		}
		if _, err := os.Stat(c0Path); err != nil {
			continue
		}
		abs, _ := filepath.Abs(c0Path)
		if compiled[abs] {
			continue
		}
		compiled[abs] = true

		goSrc, goFile, err := compileC0ModuleToGo(c0Path, cfg)
		if err != nil {
			return "", err
		}
		rel := strings.TrimPrefix(goPath, c0ProjectImportPrefix)
		depDir := filepath.Join(tmpDir, "deps", filepath.FromSlash(rel))
		if err := os.MkdirAll(depDir, 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(depDir, goFile), []byte(goSrc), 0644); err != nil {
			return "", err
		}
		depMod := fmt.Sprintf("module %s\n\ngo 1.22\n", goPath)
		if err := os.WriteFile(filepath.Join(depDir, "go.mod"), []byte(depMod), 0644); err != nil {
			return "", err
		}
		replaces = append(replaces, fmt.Sprintf("replace %s => ./deps/%s", goPath, filepath.ToSlash(rel)))
		requires = append(requires, fmt.Sprintf("require %s v0.0.0", goPath))
	}
	goMod := "module test\n\ngo 1.22\n"
	if len(requires) > 0 {
		goMod += "\n" + strings.Join(requires, "\n") + "\n"
	}
	if len(replaces) > 0 {
		goMod += "\n" + strings.Join(replaces, "\n") + "\n"
	}
	return goMod, nil
}

func runTests(dir string) int {
	pattern := filepath.Join(dir, "*_test.c0")
	files, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no test files found in %s (matching *_test.c0)\n", dir)
		return 1
	}

	cfg := loadProjectConfig(dir)

	var passed, failed int
	for _, file := range files {
		name := filepath.Base(file)
		fmt.Printf("=== RUN   %s\n", name)

		src, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("--- FAIL: %s (read error: %v)\n", name, err)
			failed++
			continue
		}

		mod, err := parser.Parse(file, src)
		if err != nil {
			fmt.Printf("--- FAIL: %s (parse: %v)\n", name, err)
			failed++
			continue
		}
		mod = desugar.DesugarModule(mod)

		tm, vtm, typeErrs := typecheck.CheckWithTypes(mod)
		if len(typeErrs) > 0 {
			fmt.Printf("--- FAIL: %s (type errors)\n", name)
			for _, e := range typeErrs {
				fmt.Printf("    %v\n", e)
			}
			failed++
			continue
		}

		gen := codegen.NewGenerator(file, cfg)
		gen.SetTypeMap(tm, vtm)
		goSrc, err := gen.Generate(mod)
		if err != nil {
			fmt.Printf("--- FAIL: %s (codegen: %v)\n", name, err)
			failed++
			continue
		}

		// Build and run in a temp directory.
		// Use a cache dir to avoid /tmp (may be noexec) and the project dir (has go.mod).
		cacheDir, _ := os.UserCacheDir()
		if cacheDir == "" {
			cacheDir = os.TempDir()
		}
		tmpDir, err := os.MkdirTemp(cacheDir, "c0-test-*")
		if err != nil {
			tmpDir, err = os.MkdirTemp("", "c0-test-*")
			if err != nil {
				fmt.Printf("--- FAIL: %s (temp dir: %v)\n", name, err)
				failed++
				continue
			}
		}
		defer os.RemoveAll(tmpDir)

		projectRoot := findProjectRoot(dir)
		goMod, err := writeOpenDependencies(mod, cfg, projectRoot, tmpDir)
		if err != nil {
			fmt.Printf("--- FAIL: %s (deps: %v)\n", name, err)
			failed++
			continue
		}
		os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644)

		// Test files must be package main to produce an executable
		goSrc = strings.Replace(goSrc, "package "+gen.GoPkg(), "package main", 1)

		outFile := gen.GoFileName()
		outPath := filepath.Join(tmpDir, outFile)
		os.WriteFile(outPath, []byte(goSrc), 0644)

		binPath := filepath.Join(tmpDir, "testbin")
		buildCmd := exec.Command("go", "build", "-o", binPath, outFile)
		buildCmd.Dir = tmpDir
		if out, err := buildCmd.CombinedOutput(); err != nil {
			fmt.Printf("--- FAIL: %s (go build)\n%s\n", name, out)
			failed++
			continue
		}

		runCmd := exec.Command(binPath)
		runCmd.Dir = tmpDir
		runOut, runErr := runCmd.CombinedOutput()
		if runErr != nil {
			fmt.Printf("--- FAIL: %s (exit %v)\n%s\n", name, runErr, runOut)
			failed++
		} else {
			fmt.Printf("--- PASS: %s\n", name)
			passed++
		}
	}

	fmt.Printf("\n%d passed, %d failed\n", passed, failed)
	if failed > 0 {
		return 1
	}
	return 0
}

// loadProjectConfig finds and loads the c0.toml for the given source file.
func loadProjectConfig(srcFile string) *config.Config {
	// Look in the directory containing the source file
	dir := filepath.Dir(srcFile)
	cfgPath := filepath.Join(dir, "c0.toml")
	cfg, err := config.LoadConfig(cfgPath)
	if err == nil && cfg != nil {
		return cfg
	}
	// Fallback: look in the current working directory
	cwd, _ := os.Getwd()
	cfgPath = filepath.Join(cwd, "c0.toml")
	cfg, err = config.LoadConfig(cfgPath)
	if err == nil && cfg != nil {
		return cfg
	}
	// Return default config
	return config.DefaultConfig()
}

// ---------------------------------------------------------------------------
// Pretty-printer (for debug/CLI)
// ---------------------------------------------------------------------------

func printModule(mod *ast.Module) string {
	return FormatModule(mod)
}

// FormatModule returns a properly formatted C0 source from an AST.
// This is the formatter used by the `c0 fmt` command.
func FormatModule(mod *ast.Module) string {
	var buf strings.Builder

	// Module header
	buf.WriteString(fmt.Sprintf("module %s\n", mod.Name))

	// Open statements
	for i, o := range mod.Opens {
		if i == 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(fmt.Sprintf("open %s\n", o.Path))
	}

	// Declarations
	for i, d := range mod.Decls {
		if i > 0 {
			buf.WriteString("\n")
		}
		formatDecl(&buf, d, 0)
	}

	return buf.String()
}

func formatDecl(buf *strings.Builder, d ast.TopDecl, depth int) {
	indent := strings.Repeat("  ", depth)
	switch d := d.(type) {
	case *ast.LetDecl:
		formatLetDecl(buf, d, indent)
	case *ast.TypeDecl:
		formatTypeDecl(buf, d, indent)
	case *ast.ExternDecl:
		buf.WriteString(fmt.Sprintf("%sextern %q %q { ... }\n", indent, d.Lang, d.Path))
	case *ast.GolangEmbedDecl:
		buf.WriteString(fmt.Sprintf("%s@golang { ... }\n", indent))
		for _, v := range d.Vals {
			buf.WriteString(fmt.Sprintf("%sval %s : ...\n", indent, v.Name))
		}
	}
}

func formatLetDecl(buf *strings.Builder, d *ast.LetDecl, indent string) {
	rec := ""
	if d.Rec {
		rec = "rec "
	}
	mut := ""
	if d.Mutable {
		mut = "mutable "
	}

	for i, b := range d.Bindings {
		if i > 0 {
			buf.WriteString(indent + "and\n")
		}

		// Build the let line
		buf.WriteString(indent + "let " + rec + mut + b.Name)

		// Function parameters
		if len(b.Params) > 0 {
			for _, p := range b.Params {
				buf.WriteString(" " + p.Name)
				if p.Type != nil {
					buf.WriteString(" : " + formatType(p.Type))
				}
			}
		}

		// Return type
		if b.RetType != nil {
			buf.WriteString(" : " + formatType(b.RetType))
		}
		buf.WriteString(" =\n")
		buf.WriteString(formatExpr(b.Body, indent+"  "))
	}
}

func formatTypeDecl(buf *strings.Builder, d *ast.TypeDecl, indent string) {
	buf.WriteString(fmt.Sprintf("%stype %s", indent, d.Name))
	for _, tp := range d.TypeParams {
		buf.WriteString(" " + tp)
	}
	if d.Quantity == 1 {
		buf.WriteString(" : 1")
	}

	switch k := d.Kind.(type) {
	case *ast.OpaqueTypeKind:
		buf.WriteByte('\n')
	case *ast.RecordTypeKind:
		buf.WriteString(" = {\n")
		for i, f := range k.Fields {
			buf.WriteString(fmt.Sprintf("%s  %s : %s", indent, f.Name, formatType(f.Type)))
			if i < len(k.Fields)-1 {
				buf.WriteString(";\n")
			} else {
				buf.WriteString("\n")
			}
		}
		buf.WriteString(indent + "}\n")
	case *ast.ADTTypeKind:
		buf.WriteString(" =\n")
		for _, c := range k.Cases {
			buf.WriteString(fmt.Sprintf("%s  | %s", indent, c.Name))
			if c.Arg != nil {
				buf.WriteString(" of " + formatType(c.Arg))
			}
			buf.WriteByte('\n')
		}
	case *ast.AliasTypeKind:
		buf.WriteString(" = " + formatType(k.Alias) + "\n")
	}
}

// formatType returns a formatted type string with proper C0 syntax
func formatType(t ast.Type) string {
	if t == nil {
		return ""
	}
	switch t := t.(type) {
	case *ast.TIdent:
		return t.Name
	case *ast.TApp:
		return formatType(t.Func) + "<" + formatType(t.Arg) + ">"
	case *ast.TFun:
		return formatType(t.From) + " -> " + formatType(t.To)
	case *ast.TTuple:
		var elems []string
		for _, e := range t.Elems {
			elems = append(elems, formatType(e))
		}
		return "(" + strings.Join(elems, " * ") + ")"
	case *ast.TRecord:
		var fields []string
		for _, f := range t.Fields {
			fields = append(fields, f.Name+": "+formatType(f.Type))
		}
		s := "{ " + strings.Join(fields, "; ") + " "
		if t.Open {
			s += "| .. "
		}
		return s + "}"
	case *ast.TVar:
		return t.Name
	case *ast.RefinementType:
		return formatType(t.Inner) + " where " + ExprStr(t.Pred)
	case *ast.TChan:
		return formatType(t.Elem) + " chan"
	default:
		return "<type>"
	}
}

// formatExpr returns a formatted expression string with proper indentation
func formatExpr(e ast.Expr, indent string) string {
	return indent + ExprStr(e) + "\n"
}

func formatPattern(p ast.Pattern) string {
	return patternStr(p) // Reuse existing patternStr function
}

func ExprStr(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.LitExpr:
		return fmt.Sprintf("%v", e.Value)
	case *ast.IdentExpr:
		return e.Name
	case *ast.ConstructorExpr:
		if e.Arg != nil {
			return e.Name + "(" + ExprStr(e.Arg) + ")"
		}
		return e.Name
	case *ast.AppExpr:
		return "(" + ExprStr(e.Func) + " " + ExprStr(e.Arg) + ")"
	case *ast.IfExpr:
		return fmt.Sprintf("(if %s then %s else %s)",
			ExprStr(e.Cond), ExprStr(e.ThenBranch), ExprStr(e.ElseBranch))
	case *ast.MatchExpr:
		var arms []string
		for _, a := range e.Arms {
			arms = append(arms, patternStr(a.Pattern)+" -> "+ExprStr(a.Body))
		}
		return "(match " + ExprStr(e.Scrutinee) + " with " + strings.Join(arms, " | ") + ")"
	case *ast.LetInExpr:
		bs := make([]string, len(e.Bindings))
		for i, b := range e.Bindings {
			bs[i] = b.Name + " = " + ExprStr(b.Body)
		}
		return "(let " + strings.Join(bs, " and ") + " in " + ExprStr(e.Body) + ")"
	case *ast.FunExpr:
		ps := make([]string, len(e.Params))
		for i, p := range e.Params {
			ps[i] = p.Name
		}
		return "(fun " + strings.Join(ps, " ") + " -> " + ExprStr(e.Body) + ")"
	case *ast.BinaryExpr:
		return fmt.Sprintf("(%s %s %s)", ExprStr(e.Left), e.Op, ExprStr(e.Right))
	case *ast.PipeExpr:
		return "(" + ExprStr(e.Left) + " |> " + ExprStr(e.Right) + ")"
	case *ast.QuestionExpr:
		if e.Arg != nil {
			return "(" + ExprStr(e.Left) + " ? " + ExprStr(e.Arg) + ")"
		}
		return "(" + ExprStr(e.Left) + " ?)"
	case *ast.RecordExpr:
		var fields []string
		for _, f := range e.Fields {
			if f.Value != nil {
				fields = append(fields, f.Name+" = "+ExprStr(f.Value))
			} else {
				fields = append(fields, f.Name)
			}
		}
		return "{" + strings.Join(fields, "; ") + "}"
	case *ast.RecordUpdateExpr:
		var fields []string
		for _, f := range e.Fields {
			fields = append(fields, f.Name+" = "+ExprStr(f.Value))
		}
		return "{" + ExprStr(e.Base) + " with " + strings.Join(fields, "; ") + "}"
	case *ast.FieldAccessExpr:
		return ExprStr(e.Left) + "." + e.Field
	case *ast.TupleExpr:
		var elems []string
		for _, el := range e.Elems {
			elems = append(elems, ExprStr(el))
		}
		return "(" + strings.Join(elems, ", ") + ")"
	case *ast.ListExpr:
		var elems []string
		for _, el := range e.Elems {
			elems = append(elems, ExprStr(el))
		}
		return "[" + strings.Join(elems, "; ") + "]"
	case *ast.ParenExpr:
		return "(" + ExprStr(e.Inner) + ")"
	default:
		return "<expr>"
	}
}

func patternStr(p ast.Pattern) string {
	switch p := p.(type) {
	case *ast.WildcardPattern:
		return "_"
	case *ast.IdentPattern:
		return p.Name
	case *ast.LitPattern:
		return fmt.Sprintf("%v", p.Value)
	case *ast.ConstructorPattern:
		if p.Arg != nil {
			return p.Name + "(" + patternStr(p.Arg) + ")"
		}
		return p.Name
	case *ast.RecordPattern:
		var fields []string
		for _, f := range p.Fields {
			if f.Pattern != nil {
				fields = append(fields, f.Name+" = "+patternStr(f.Pattern))
			} else {
				fields = append(fields, f.Name)
			}
		}
		return "{" + strings.Join(fields, "; ") + "}"
	case *ast.TuplePattern:
		var elems []string
		for _, el := range p.Elems {
			elems = append(elems, patternStr(el))
		}
		return "(" + strings.Join(elems, ", ") + ")"
	case *ast.ListPattern:
		if len(p.Elems) == 0 {
			return "[]"
		}
		var elems []string
		for _, el := range p.Elems {
			elems = append(elems, patternStr(el))
		}
		return "[" + strings.Join(elems, "; ") + "]"
	case *ast.ConsPattern:
		return patternStr(p.Head) + " :: " + patternStr(p.Tail)
	case *ast.AliasPattern:
		return patternStr(p.Pattern) + " as " + p.Name
	default:
		return "<pat>"
	}
}

func typeStr(t ast.Type) string {
	if t == nil {
		return ""
	}
	switch t := t.(type) {
	case *ast.TIdent:
		return t.Name
	case *ast.TApp:
		return typeStr(t.Func) + "(" + typeStr(t.Arg) + ")"
	case *ast.TFun:
		return typeStr(t.From) + " -> " + typeStr(t.To)
	case *ast.TTuple:
		var elems []string
		for _, e := range t.Elems {
			elems = append(elems, typeStr(e))
		}
		return "(" + strings.Join(elems, " * ") + ")"
	case *ast.TRecord:
		var fields []string
		for _, f := range t.Fields {
			fields = append(fields, f.Name+": "+typeStr(f.Type))
		}
		return "{" + strings.Join(fields, "; ") + "}"
	case *ast.TVar:
		return t.Name
	case *ast.RefinementType:
		return typeStr(t.Inner) + " where " + ExprStr(t.Pred)
	default:
		return "<type>"
	}
}

// buildLinearTypes extracts the set of linear type names from a module.
func buildLinearTypes(mod *ast.Module) map[string]bool {
	lt := make(map[string]bool)
	for _, d := range mod.Decls {
		td, ok := d.(*ast.TypeDecl)
		if !ok {
			continue
		}
		if td.Quantity == 1 {
			lt[td.Name] = true
		}
	}
	// Built-in linear types (not declared in user modules).
	lt["owned_chan"] = true
	return lt
}
