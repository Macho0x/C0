// Command goop is the Goop language compiler CLI.
//
// Usage:
//
//	goop lex  <file.goop>    lexical analysis — print token stream
//	goop parse <file.goop>    parse — pretty-print AST
//	goop check <file.goop>    parse and report success/failure
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

	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/checkpipeline"
	"goop.dev/compiler/internal/codegen"
	"goop.dev/compiler/internal/color"
	"goop.dev/compiler/internal/config"
	"goop.dev/compiler/internal/desugar"
	"goop.dev/compiler/internal/effects"
	gfmt "goop.dev/compiler/internal/fmt"
	lc0 "goop.dev/compiler/internal/lexer"
	"goop.dev/compiler/internal/modresolve"
	"goop.dev/compiler/internal/parser"
	"goop.dev/compiler/internal/refine"
	"goop.dev/compiler/internal/report"
	"goop.dev/compiler/internal/token"
	"goop.dev/compiler/internal/typecheck"
	"goop.dev/compiler/internal/typeinfo"
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

// lspResult builds a JSON-RPC response. A nil result encodes as JSON null;
// without that, omitempty would omit "result" and break LSP clients.
func lspResult(id interface{}, result interface{}) Response {
	resp := Response{Jsonrpc: "2.0", ID: id}
	if result == nil {
		resp.Result = json.RawMessage("null")
	} else {
		resp.Result = result
	}
	return resp
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
		switch a {
		case "--no-source-map":
			globalWriteMap = false
		case "--color":
			colorOutput = true
		case "-i", "--in-place":
			inPlace = true
		default:
			filtered = append(filtered, a)
		}
	}

	if len(filtered) < 2 && (len(filtered) == 0 || filtered[0] != "test") && filtered[0] != "lsp" && filtered[0] != "fmt" {
		fmt.Fprintf(os.Stderr, "Usage: goop [--no-source-map] [--color] [-i] <command> <file.goop>\n")
		fmt.Fprintf(os.Stderr, "Commands: lex, parse, check, compile, build, test, get, resolve, lsp, fmt (format)\n")
		os.Exit(1)
	}

	cmd := filtered[0]

	// `goop test` and `goop lsp` are special: they don't need a file argument
	if cmd == "test" {
		dir := "."
		if len(filtered) >= 2 {
			dir = filtered[1]
		}
		os.Exit(runTests(dir))
	}
	if cmd == "get" {
		os.Exit(runGet(filtered[1:]))
	}
	if cmd == "lsp" {
		runLSP()
		return
	}

	file := filtered[1]

	// Load project config (look for goop.toml near the source file or CWD)
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

		lock := loadProjectLock(file)
		tm, _, typeErrors := typecheck.CheckWithTypesForFile(mod, file, cfg, lock)
		if len(typeErrors) > 0 {
			fmt.Println("FAIL: type errors:")
			for _, e := range typeErrors {
				fmt.Print(report.Render(e, src))
			}
			os.Exit(1)
		}

		if _, _, warns, fatal := runSafetyChecks(mod, tm, src, cfg); fatal {
			os.Exit(1)
		} else if len(warns) > 0 {
			fmt.Println("WARN: safety warnings:")
			for _, w := range warns {
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
		tm, vtm, typeErrs := typecheckModule(mod, file, cfg)
		if len(typeErrs) > 0 {
			fmt.Println("FAIL: type errors:")
			for _, e := range typeErrs {
				fmt.Print(report.Render(e, src))
			}
			os.Exit(1)
		}

		// Safety checks (linear, nilchan, refine, exhaust)
		proven, funcProven, warns, fatal := runSafetyChecks(mod, tm, src, cfg)
		if fatal {
			os.Exit(1)
		}
		for _, w := range warns {
			fmt.Fprintf(os.Stderr, "WARNING: %v\n", w)
		}

		// Generate Go code
		mod = effects.TransformCPS(mod)
		gen := codegen.NewGenerator(file, cfg)
		gen.SetTypeMap(tm, vtm)
		gen.SetProvenSites(proven)
		gen.SetRefinementMeta(funcProven)
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
		tm, vtm, typeErrs := typecheckModule(mod, file, cfg)
		if len(typeErrs) > 0 {
			fmt.Println("FAIL: type errors:")
			for _, e := range typeErrs {
				fmt.Print(report.Render(e, src))
			}
			os.Exit(1)
		}

		// Safety checks (linear, nilchan, refine, exhaust)
		proven, funcProven, warns, fatal := runSafetyChecks(mod, tm, src, cfg)
		if fatal {
			os.Exit(1)
		}
		for _, w := range warns {
			fmt.Fprintf(os.Stderr, "WARNING: %v\n", w)
		}

		// Generate Go code
		mod = effects.TransformCPS(mod)
		gen := codegen.NewGenerator(file, cfg)
		gen.SetTypeMap(tm, vtm)
		gen.SetProvenSites(proven)
		gen.SetRefinementMeta(funcProven)
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
			modContent := "module goopbuild\n\ngo 1.22\n"
			os.WriteFile(goModPath, []byte(modContent), 0644)
			fmt.Printf("created %s (temporary)\n", goModPath)
		}

		var cmd *exec.Cmd
		if isMixed || hasGoMod {
			// Build the whole package (includes hand-written .go files)
			cmd = exec.Command("go", "build", ".")
		} else {
			cmd = exec.Command("go", "build", "-o", "goop-out", genFile)
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
		lock := loadProjectLock(file)
		root := modresolve.FindProjectRoot(file)
		r := modresolve.New(cfg, lock, root)
		if len(mod.Imports) == 0 {
			fmt.Println("(no imports)")
		}
		for _, spec := range mod.Imports {
			switch spec.Kind {
			case ast.ImportGolang:
				alias := spec.Alias
				if alias == "" {
					alias = "(default)"
				}
				fmt.Printf("import golang %-12s %q", alias, spec.Path)
				if len(spec.Vals) > 0 {
					fmt.Printf(" (%d val bindings)", len(spec.Vals))
				}
				fmt.Println()
			case ast.ImportGoop:
				resolved, err := r.ResolveGoopPath(spec.Path)
				if err != nil {
					fmt.Printf("import goop %q → ERROR: %v\n", spec.Path, err)
					continue
				}
				alias := modresolve.ImportAlias(spec, resolved)
				fmt.Printf("import goop %-12s %q → %s (package %s)\n", alias, spec.Path, resolved.GoImportPath, resolved.PkgName)
				if resolved.SourceFile != "" {
					fmt.Printf("  source: %s\n", resolved.SourceFile)
				}
			}
		}
		deps, err := r.LoadModuleGraph(file, mod)
		if err != nil {
			fmt.Fprintf(os.Stderr, "import graph error: %v\n", err)
			os.Exit(1)
		}
		if len(deps) > 1 {
			fmt.Println("\nimport graph:")
			for path := range deps {
				fmt.Printf("  %s\n", path)
			}
		}

	case "fmt":
		mod, err := parser.Parse(file, src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
			os.Exit(1)
		}
		formatted := gfmt.FormatModule(mod)
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
		fmt.Fprintf(os.Stderr, "Commands: lex, parse, check, compile, build, test, get, resolve, lsp, fmt\n")
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
	return lspResult(req.ID, InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync:   1,
			HoverProvider:      true,
			DefinitionProvider: true,
			CompletionProvider: true,
		},
	})
}

func (s *LSPServer) handleDidSave(req Request) Response {
	s.handleDocumentUpdate(req.Params)
	return lspResult(req.ID, nil)
}

func (s *LSPServer) handleDidOpen(req Request) Response {
	s.handleDocumentUpdate(req.Params)
	return lspResult(req.ID, nil)
}

func (s *LSPServer) handleDidChange(req Request) Response {
	s.handleDocumentUpdate(req.Params)
	return lspResult(req.ID, nil)
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
		diagnostics = append(diagnostics, diagnosticFromError(parseErr))
	} else {
		mod = desugar.DesugarModule(mod)

		lspCfg := loadProjectConfig(fileName)
		tm, vtm, typeErrs := typecheckModule(mod, fileName, lspCfg)
		for _, terr := range typeErrs {
			diagnostics = append(diagnostics, diagnosticFromError(terr))
		}

		for _, terr := range lspSafetyDiagnostics(mod, tm, lspCfg) {
			diagnostics = append(diagnostics, diagnosticFromError(terr))
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
func diagnosticFromError(err error) Diagnostic {
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
		return lspResult(req.ID, nil)
	}

	// Find the expression at this position
	hoverText, found := s.findHoverInfo(state, params.Position)
	if !found {
		return lspResult(req.ID, nil)
	}

	return lspResult(req.ID, HoverResponse{
		Contents: HoverContents{
			Kind:  "plaintext",
			Value: hoverText,
		},
	})
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
		return lspResult(req.ID, nil)
	}

	loc := s.findDefinition(state, params.Position)
	if loc == nil {
		return lspResult(req.ID, nil)
	}

	loc.URI = params.TextDocument.URI
	return lspResult(req.ID, loc)
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
		return lspResult(req.ID, nil)
	}

	items := s.collectCompletions(state)
	return lspResult(req.ID, CompletionList{
		IsIncomplete: false,
		Items:        items,
	})
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
	if err := scanner.Err(); err != nil {
		return req, err
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

const goopProjectImportPrefix = "github.com/Macho0x/Goop/"

func findProjectRoot(start string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "goop.toml")); err == nil {
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

func compileGoopModuleToGo(goopPath string, cfg *config.Config) (string, string, error) {
	src, err := os.ReadFile(goopPath)
	if err != nil {
		return "", "", err
	}
	mod, err := parser.Parse(goopPath, src)
	if err != nil {
		return "", "", err
	}
	mod = desugar.DesugarModule(mod)
	tm, vtm, typeErrs := typecheckModule(mod, goopPath, cfg)
	if len(typeErrs) > 0 {
		return "", "", fmt.Errorf("type errors in %s: %v", goopPath, typeErrs[0])
	}
	mod = effects.TransformCPS(mod)
	gen := codegen.NewGenerator(goopPath, cfg)
	gen.SetTypeMap(tm, vtm)
	goSrc, err := gen.Generate(mod)
	if err != nil {
		return "", "", err
	}
	return goSrc, gen.GoFileName(), nil
}

func writeImportDependencies(mod *ast.Module, cfg *config.Config, projectRoot, tmpDir, entryFile string) (string, error) {
	if projectRoot == "" {
		return "module test\n\ngo 1.22\n", nil
	}
	lock, _ := config.LoadLockfile(filepath.Join(projectRoot, "goop.lock"))
	r := modresolve.New(cfg, lock, projectRoot)
	sources := make(map[string]string)

	var collect func(*ast.Module) error
	collect = func(m *ast.Module) error {
		for _, spec := range m.Imports {
			if spec.Kind != ast.ImportGoop {
				continue
			}
			resolved, err := r.ResolveGoopPath(spec.Path)
			if err != nil {
				return err
			}
			if resolved.SourceFile == "" {
				return fmt.Errorf("module %q not found", spec.Path)
			}
			if _, ok := sources[resolved.SourceFile]; ok {
				continue
			}
			sources[resolved.SourceFile] = resolved.GoImportPath
			src, err := os.ReadFile(resolved.SourceFile)
			if err != nil {
				return err
			}
			depMod, err := parser.Parse(resolved.SourceFile, src)
			if err != nil {
				return err
			}
			depMod = desugar.DesugarModule(depMod)
			if err := collect(depMod); err != nil {
				return err
			}
		}
		return nil
	}
	if err := collect(mod); err != nil {
		return "", err
	}
	if len(sources) == 0 {
		return "module test\n\ngo 1.22\n", nil
	}

	var replaces []string
	var requires []string
	for goopPath, goPath := range sources {
		goSrc, goFile, err := compileGoopModuleToGo(goopPath, cfg)
		if err != nil {
			return "", err
		}
		rel := strings.TrimPrefix(goPath, goopProjectImportPrefix)
		depDir := filepath.Join(tmpDir, "deps", filepath.FromSlash(rel))
		if err := os.MkdirAll(depDir, 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(depDir, goFile), []byte(goSrc), 0644); err != nil {
			return "", err
		}
		depModContent := fmt.Sprintf("module %s\n\ngo 1.22\n", goPath)
		if err := os.WriteFile(filepath.Join(depDir, "go.mod"), []byte(depModContent), 0644); err != nil {
			return "", err
		}
		replaces = append(replaces, fmt.Sprintf("replace %s => ./deps/%s", goPath, filepath.ToSlash(rel)))
		requires = append(requires, fmt.Sprintf("require %s v0.0.0", goPath))
	}
	_ = entryFile
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
	pattern := filepath.Join(dir, "*_test.goop")
	files, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no test files found in %s (matching *_test.goop)\n", dir)
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

		tm, vtm, typeErrs := typecheckModule(mod, file, cfg)
		if len(typeErrs) > 0 {
			fmt.Printf("--- FAIL: %s (type errors)\n", name)
			for _, e := range typeErrs {
				fmt.Printf("    %v\n", e)
			}
			failed++
			continue
		}

		proven, funcProven, warns, fatal := runSafetyChecks(mod, tm, src, cfg)
		if fatal {
			fmt.Printf("--- FAIL: %s (safety errors)\n", name)
			failed++
			continue
		}
		for _, w := range warns {
			fmt.Printf("    WARNING: %v\n", w)
		}

		gen := codegen.NewGenerator(file, cfg)
		gen.SetTypeMap(tm, vtm)
		gen.SetProvenSites(proven)
		gen.SetRefinementMeta(funcProven)
		mod = effects.TransformCPS(mod)
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
		tmpDir, err := os.MkdirTemp(cacheDir, "goop-test-*")
		if err != nil {
			tmpDir, err = os.MkdirTemp("", "goop-test-*")
			if err != nil {
				fmt.Printf("--- FAIL: %s (temp dir: %v)\n", name, err)
				failed++
				continue
			}
		}
		defer os.RemoveAll(tmpDir)

		projectRoot := findProjectRoot(dir)
		goMod, err := writeImportDependencies(mod, cfg, projectRoot, tmpDir, file)
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

// loadProjectConfig finds and loads the goop.toml for the given source file.
func loadProjectConfig(srcFile string) *config.Config {
	// Look in the directory containing the source file
	dir := filepath.Dir(srcFile)
	cfgPath := filepath.Join(dir, "goop.toml")
	cfg, err := config.LoadConfig(cfgPath)
	if err == nil && cfg != nil {
		return cfg
	}
	// Fallback: look in the current working directory
	cwd, _ := os.Getwd()
	cfgPath = filepath.Join(cwd, "goop.toml")
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
	return gfmt.FormatModule(mod)
}

func loadProjectLock(file string) *config.Lockfile {
	root := modresolve.FindProjectRoot(file)
	if root == "" {
		lf, _ := config.LoadLockfile("goop.lock")
		return lf
	}
	lf, _ := config.LoadLockfile(filepath.Join(root, "goop.lock"))
	return lf
}

func typecheckModule(mod *ast.Module, file string, cfg *config.Config) (typeinfo.TypeMap, typeinfo.VarTypeMap, []error) {
	return typecheck.CheckWithTypesForFile(mod, file, cfg, loadProjectLock(file))
}

// runSafetyChecks runs linear, nil-channel, refinement, and exhaustiveness passes.
// Returns proven refinement sites for codegen, non-fatal warnings, and whether any fatal error occurred.
func runSafetyChecks(mod *ast.Module, tm typeinfo.TypeMap, src []byte, cfg *config.Config) (proven refine.ProvenSites, funcAllProven map[string]bool, warnings []error, fatal bool) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	checkpipeline.RegisterADTsFromModule(mod)
	linearTypes := buildLinearTypes(mod)
	r := checkpipeline.Run(mod, tm, linearTypes, cfg)

	if len(r.LinearErrors) > 0 {
		fmt.Println("FAIL: linear discharge errors:")
		for _, e := range r.LinearErrors {
			fmt.Print(report.Render(e, src))
		}
		fatal = true
	}
	warnings = append(warnings, r.LinearWarnings...)
	if len(r.ChannelRaceErrors) > 0 {
		fmt.Println("FAIL: channel-mediated race errors:")
		for _, e := range r.ChannelRaceErrors {
			fmt.Print(report.Render(e, src))
		}
		fatal = true
	}
	warnings = append(warnings, r.ChannelRaceWarns...)
	if len(r.DeadlockErrors) > 0 {
		fmt.Println("FAIL: deadlock errors:")
		for _, e := range r.DeadlockErrors {
			fmt.Print(report.Render(e, src))
		}
		fatal = true
	}
	warnings = append(warnings, r.DeadlockWarns...)
	if len(r.NilchanErrors) > 0 {
		fmt.Println("FAIL: nil-channel errors:")
		for _, e := range r.NilchanErrors {
			fmt.Print(report.Render(e, src))
		}
		fatal = true
	}
	warnings = append(warnings, r.RefineWarnings...)
	if len(r.RefineErrors) > 0 {
		fmt.Println("FAIL: refinement errors:")
		for _, e := range r.RefineErrors {
			fmt.Print(report.Render(e, src))
		}
		fatal = true
	}
	if len(r.ExhaustErrors) > 0 {
		fmt.Println("FAIL: exhaustiveness errors:")
		for _, e := range r.ExhaustErrors {
			fmt.Print(report.Render(e, src))
		}
		fatal = true
	}
	warnings = append(warnings, r.ExhaustWarns...)
	return r.RefineProven, r.RefineFuncProven, warnings, fatal
}

// lspSafetyDiagnostics returns safety check diagnostics for the LSP (errors only).
func lspSafetyDiagnostics(mod *ast.Module, tm typeinfo.TypeMap, cfg *config.Config) []error {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	checkpipeline.RegisterADTsFromModule(mod)
	linearTypes := buildLinearTypes(mod)
	r := checkpipeline.Run(mod, tm, linearTypes, cfg)
	var out []error
	out = append(out, r.LinearErrors...)
	out = append(out, r.ChannelRaceErrors...)
	out = append(out, r.DeadlockErrors...)
	out = append(out, r.NilchanErrors...)
	out = append(out, r.RefineErrors...)
	out = append(out, r.ExhaustErrors...)
	for _, w := range r.LinearWarnings {
		if cfg.Check.Concurrent == config.SeverityError {
			out = append(out, w)
		}
	}
	for _, w := range r.ChannelRaceWarns {
		if cfg.Check.Concurrent == config.SeverityError {
			out = append(out, w)
		}
	}
	for _, w := range r.DeadlockWarns {
		if cfg.Check.Deadlock == config.SeverityError {
			out = append(out, w)
		}
	}
	for _, w := range r.RefineWarnings {
		if cfg.Check.RefinementUnproven == config.SeverityError {
			out = append(out, w)
		}
	}
	for _, w := range r.ExhaustWarns {
		if cfg.Check.ExhaustRedundant == config.SeverityError {
			out = append(out, w)
		}
	}
	return out
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
