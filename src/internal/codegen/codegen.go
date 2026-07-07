// Package codegen emits idiomatic Go source from a typed C0 AST.
//
// Lowering strategy (see docs/design/04-go-lowering.md):
//   - ADTs → Go interface + one concrete struct per variant + constructors
//   - Records → Go structs with exported fields
//   - Pattern matching → Go type switches with default panic
//   - Option<T> / Result<T,E> → tagged structs with IsOk/MustOk methods
//   - Lists → Go slices []T
//   - Tuples → generated structs
//   - Curried functions → multi-parameter Go functions
//   - `?` operator → Go if err != nil { return err } idiom
//   - Pipelines |> → nested function calls
//   - Source maps → //line directives
package codegen

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"c0.dev/compiler/internal/active"
	"c0.dev/compiler/internal/ast"
	"c0.dev/compiler/internal/config"
	"c0.dev/compiler/internal/prelude"
	"c0.dev/compiler/internal/refine"
	"c0.dev/compiler/internal/token"
	"c0.dev/compiler/internal/typeinfo"
	"c0.dev/compiler/internal/types"
)

// Generator emits Go source code for a C0 module.
type Generator struct {
	buf       strings.Builder
	indent    int
	srcFile   string // original C0 source file path

	// source-map tracking
	srcMap  *SourceMap
	goLine  int // current Go output line (1-based)
	goCol   int // current Go output column (1-based)

	// module state
	moduleName  string // C0 module name
	goPkg       string // Go package name
	goFileName  string // suggested output file name

	// type tracking
	adts     map[string]*ast.TypeDecl // ADT declarations
	records  map[string]*ast.TypeDecl // record declarations
	opaqueTypes map[string]*ast.TypeDecl // opaque (linear) type declarations
	usedOption map[string]string      // Go type name → element Go type
	usedResult map[string][]string    // Go type name → [okGoType, errGoType]
	usedTuple  map[string][]string     // Go type name → field Go type names
	funcRetType map[string]string      // C0 func name → Go return type (for ?)
	funcParamCount map[string]int       // C0 func name → number of parameters
	funcParamTypes map[string][]string  // C0 func name → Go types of parameters

	// inferred types from typechecker (for polymorphic codegen)
	typeMap    typeinfo.TypeMap
	varTypeMap typeinfo.VarTypeMap

	// name mapping
	c0ToGo map[string]string // C0 name → Go name
	goToC0 map[string]string // Go name → C0 name (reverse)

	// module / import resolution
	cfg          *config.Config       // project configuration
	resolvedImports map[string]string // C0 module name → Go import path
	importPkgs  map[string]string    // Go import path → Go package name

	// prelude bindings
	prelude *prelude.Prelude

	// extern tracking
	externImports map[string]string // Go import path → package name
	externNames   map[string]string // C0 name → Go qualified name (pkg.Name)

	// row-polymorphic function params: funcName → field names
	rowParams     map[string][]string
	rowParamName  map[string]string     // funcName → row param name

	currentFunc string // function being generated (for ? operator)
	varCounter  int    // for generating unique tmp variable names
	needFmt     bool   // whether to import "fmt"
	usedChan    bool   // whether any channel operations are used in this module

	// refinement contract tracking
	provenSites       refine.ProvenSites      // call sites proven safe by refinement solver
	refinementParams  map[string][]refinedParam // func name → refinement-annotated params
}

// refinedParam stores information about a refinement-annotated parameter.
type refinedParam struct {
	index int                  // parameter index
	name  string               // parameter name
	rt    *ast.RefinementType  // the refinement type
}

// NewGenerator creates a new code generator.
func NewGenerator(srcFile string, cfg *config.Config) *Generator {
	return &Generator{
		srcFile:     srcFile,
		goLine:      1,
		goCol:       1,
		cfg:         cfg,
		prelude:     prelude.Default(),
		externImports: make(map[string]string),
		externNames:   make(map[string]string),
		rowParams:     make(map[string][]string),
		rowParamName:  make(map[string]string),
		resolvedImports: make(map[string]string),
		importPkgs:  make(map[string]string),
		adts:        make(map[string]*ast.TypeDecl),
		records:     make(map[string]*ast.TypeDecl),
		opaqueTypes: make(map[string]*ast.TypeDecl),
		usedOption:  make(map[string]string),
		usedResult:  make(map[string][]string),
		usedTuple:   make(map[string][]string),
		funcRetType: make(map[string]string),
		funcParamCount: make(map[string]int),
		funcParamTypes: make(map[string][]string),
		c0ToGo:      make(map[string]string),
		goToC0:      make(map[string]string),
		refinementParams: make(map[string][]refinedParam),
	}
}

// GoFileName returns the suggested output .go file name.
func (g *Generator) GoFileName() string {
	return g.goFileName
}

// GoPkg returns the generated Go package name.
func (g *Generator) GoPkg() string {
	return g.goPkg
}

// SourceMap returns the accumulated source-map data.
func (g *Generator) SourceMap() *SourceMap {
	return g.srcMap
}

// SetTypeMap stores the type-map produced by the type checker so that
// codegen can use inferred (resolved) types when lowering polymorphic
// operations such as channel creation.
func (g *Generator) SetTypeMap(tm typeinfo.TypeMap, vtm typeinfo.VarTypeMap) {
	g.typeMap = tm
	g.varTypeMap = vtm
}

// SetProvenSites stores the proven call sites from the refinement checker.
// Call sites in this set have had their refinement contracts proven at
// compile time, so codegen can skip emitting runtime panic guards for them.
func (g *Generator) SetProvenSites(proven refine.ProvenSites) {
	g.provenSites = proven
}

// typeOf returns the inferred type for an expression node, or nil if
// no type was recorded.
func (g *Generator) typeOf(node ast.Expr) types.Type {
	if g.typeMap == nil {
		return nil
	}
	return g.typeMap[node]
}

// internalTypeToGo converts an internal types.Type to a Go type string.
// It handles the basic set of types that can appear as channel element types.
func (g *Generator) internalTypeToGo(t types.Type) string {
	if t == nil {
		return "interface{}"
	}
	switch t := t.(type) {
	case *types.Prim:
		switch t.Name {
		case "int":
			return "int"
		case "float":
			return "float64"
		case "bool":
			return "bool"
		case "string":
			return "string"
		case "unit":
			return "struct{}"
		default:
			return t.Name
		}
	case *types.TChan:
		return "*C0Chan"
	case *types.TVar:
		return "interface{}"
	case *types.TCon:
		if t.Name == "list" && len(t.Args) > 0 {
			return "[]" + g.internalTypeToGo(t.Args[0])
		}
		if t.Name == "option" && len(t.Args) > 0 {
			return "Option" + exported(g.internalTypeToGo(t.Args[0]))
		}
		if t.Name == "result" && len(t.Args) > 0 {
			arg := g.internalTypeToGo(t.Args[0])
			return "Result" + exported(arg)
		}
		return "interface{}"
	case *types.TAdt:
		if t.Name == "owned_chan" {
			return "*C0Chan"
		}
		return "interface{}"
	default:
		return "interface{}"
	}
}

// recordMapping records a C0→Go position mapping at the current Go output position.
func (g *Generator) recordMapping(c0Line, c0Col int) {
	if g.srcMap != nil {
		g.srcMap.Add(c0Line, c0Col, g.goLine, g.goCol)
	}
}

// Generate produces Go source code from a C0 module.
func (g *Generator) Generate(mod *ast.Module) (string, error) {
	g.moduleName = mod.Name
	g.goPkg = goPkgName(mod.Name)
	g.goFileName = goFileName(mod.Name)

	// Initialise source map (generated path is the Go file, source is the C0 file)
	g.srcMap = NewSourceMap(g.srcFile, g.goFileName)

	// Register C0 → Go name mappings
	g.c0ToGo["int"] = "int"
	g.c0ToGo["float"] = "float64"
	g.c0ToGo["bool"] = "bool"
	g.c0ToGo["string"] = "string"
	g.c0ToGo["unit"] = "struct{}"

	// Pre-scan: collect type and function declarations
	g.prescan(mod)

	// Resolve open statements to Go imports
	g.resolveOpens(mod)

	// Collect extern imports
	g.collectExterns(mod)

	// Build the body first so we know what imports are needed
	var body strings.Builder
	origBuf := g.buf
	g.buf = body

	// Tuple type definitions
	g.emitTupleTypes()
	// Option/Result type definitions
	g.emitOptionTypes()
	g.emitResultTypes()
	// Record type definitions
	g.emitRecordTypes()
	// ADT type definitions
	g.emitADTTypes()
	// Opaque type definitions (linear types erased in Go)
	g.emitOpaqueTypes()

	// Top-level value declarations
	for _, d := range mod.Decls {
		switch d := d.(type) {
		case *ast.LetDecl:
			g.emitLetDecl(d)
		case *ast.ExternDecl:
			g.emitExternDecl(d)
		}
	}

	// Emit C0Chan wrapper if channels are used
	g.emitChanHelpers()

	bodyStr := g.buf.String()
	g.buf = origBuf

	// Emit header with source map
	g.emitLine(1, 1)
	g.emitf("// generated by c0; do not edit\n")
	g.emitf("package %s\n\n", g.goPkg)

	// Imports (now we know if fmt is needed)
	g.emitImports()

	// Body
	g.writeStr(bodyStr)

	return g.buf.String(), nil
}

// ---------------------------------------------------------------------------
// Pre-scan: gather type declarations, function return types
// ---------------------------------------------------------------------------

func (g *Generator) prescan(mod *ast.Module) {
	for _, d := range mod.Decls {
		switch d := d.(type) {
		case *ast.TypeDecl:
			switch d.Kind.(type) {
			case *ast.ADTTypeKind:
				g.adts[d.Name] = d
				goName := g.goName(d.Name)
				g.c0ToGo[d.Name] = goName
			case *ast.RecordTypeKind:
				g.records[d.Name] = d
				goName := g.goName(d.Name)
				g.c0ToGo[d.Name] = goName
			case *ast.OpaqueTypeKind:
				// Opaque linear type — erased in Go output.
				// Register the Go name so references resolve correctly.
				g.opaqueTypes[d.Name] = d
				goName := g.goName(d.Name)
				g.c0ToGo[d.Name] = goName
			case *ast.AliasTypeKind:
				// Aliases map to their underlying type
			}
		case *ast.LetDecl:
			for _, b := range d.Bindings {
				if b.RetType != nil {
					g.funcRetType[b.Name] = g.typeToGo(b.RetType)
				}
				// Filter empty-named params
				realCount := 0
				for _, p := range b.Params {
					if p.Name != "" {
						realCount++
					}
				}
				g.funcParamCount[b.Name] = realCount
				// Detect row-polymorphic parameters
				for _, p := range b.Params {
					if p.Type != nil {
						if rt, ok := p.Type.(*ast.TRecord); ok && rt.Open {
							var fields []string
							for _, f := range rt.Fields {
								fields = append(fields, f.Name)
							}
							g.rowParams[b.Name] = fields
							g.rowParamName[b.Name] = p.Name
						}
					}
				}
				// Store parameter Go types for partial application
				paramTypes := make([]string, 0)
				for _, p := range b.Params {
					if p.Name != "" {
						if p.Type != nil {
							paramTypes = append(paramTypes, g.typeToGo(p.Type))
						} else {
							paramTypes = append(paramTypes, "interface{}")
						}
					}
				}
				g.funcParamTypes[b.Name] = paramTypes
			}
		}
	}

	// Scan for used option/result/tuple types in return type annotations
	for _, d := range mod.Decls {
		if ld, ok := d.(*ast.LetDecl); ok {
			for _, b := range ld.Bindings {
				if b.RetType != nil {
					g.scanUsedTypes(b.RetType)
				}
				// Also scan parameter types
				for _, p := range b.Params {
					if p.Type != nil {
						g.scanUsedTypes(p.Type)
					}
				}
				// Scan expression for literals that imply types
				g.scanExprTypes(b.Body)
			}
		}
	}
}

func (g *Generator) scanUsedTypes(at ast.Type) {
	if at == nil {
		return
	}
	switch t := at.(type) {
	case *ast.TApp:
		// Check for option/result/list type constructors
		if ident, ok := t.Func.(*ast.TIdent); ok {
			switch ident.Name {
			case "option":
				elemGo := g.typeToGo(t.Arg)
				goType := "Option" + exported(elemGo)
				g.usedOption[goType] = elemGo
			case "result":
				elemGo := g.typeToGo(t.Arg)
				goType := "Result" + exported(elemGo)
				// Store [okType, errType] separately
				okType, errType := "interface{}", "interface{}"
				if tup, ok := t.Arg.(*ast.TTuple); ok && len(tup.Elems) >= 2 {
					okType = g.typeToGo(tup.Elems[0])
					errType = g.typeToGo(tup.Elems[1])
				}
				g.usedResult[goType] = []string{okType, errType}
			}
		}
		g.scanUsedTypes(t.Func)
		g.scanUsedTypes(t.Arg)
	case *ast.TTuple:
		if len(t.Elems) >= 2 {
			parts := make([]string, len(t.Elems))
			for i, e := range t.Elems {
				parts[i] = g.typeToGo(e)
			}
			goType := "Tuple" + strings.Join(parts, "")
			g.usedTuple[goType] = parts
		}
		for _, e := range t.Elems {
			g.scanUsedTypes(e)
		}
	case *ast.TFun:
		g.scanUsedTypes(t.From)
		g.scanUsedTypes(t.To)
	case *ast.TRecord:
		for _, f := range t.Fields {
			g.scanUsedTypes(f.Type)
		}
	case *ast.RefinementType:
		g.scanUsedTypes(t.Inner)
	case *ast.TChan:
		g.scanUsedTypes(t.Elem)
	}
}

func (g *Generator) scanExprTypes(e ast.Expr) {
	if e == nil {
		return
	}
	switch e := e.(type) {
	case *ast.IfExpr:
		g.scanExprTypes(e.ThenBranch)
		g.scanExprTypes(e.ElseBranch)
	case *ast.MatchExpr:
		for _, arm := range e.Arms {
			g.scanExprTypes(arm.Body)
		}
	case *ast.LetInExpr:
		for _, b := range e.Bindings {
			g.scanExprTypes(b.Body)
		}
		g.scanExprTypes(e.Body)
	case *ast.FunExpr:
		g.scanExprTypes(e.Body)
	case *ast.BinaryExpr:
		g.scanExprTypes(e.Left)
		g.scanExprTypes(e.Right)
	case *ast.AppExpr:
		g.scanExprTypes(e.Func)
		g.scanExprTypes(e.Arg)
	case *ast.TupleExpr:
		for _, el := range e.Elems {
			g.scanExprTypes(el)
		}
	}
}

// ---------------------------------------------------------------------------
// Name mapping helpers
// ---------------------------------------------------------------------------

func (g *Generator) goName(c0Name string) string {
	if mapped, ok := g.c0ToGo[c0Name]; ok {
		return mapped
	}
	return exported(c0Name)
}

func exported(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

func unexported(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[size:]
}

func goPkgName(modulePath string) string {
	// Last segment, lowercased
	parts := strings.Split(modulePath, ".")
	return strings.ToLower(parts[len(parts)-1])
}

func goFileName(modulePath string) string {
	parts := strings.Split(modulePath, ".")
	return strings.ToLower(parts[len(parts)-1]) + ".go"
}

// ---------------------------------------------------------------------------
// Type → Go type string
// ---------------------------------------------------------------------------

// flattenFunFrom extracts the argument types from a curried function type chain.
func (g *Generator) flattenFunFrom(t *ast.TFun) []string {
	var result []string
	current := ast.Type(t)
	for {
		if fn, ok := current.(*ast.TFun); ok {
			result = append(result, g.typeToGo(fn.From))
			current = fn.To
		} else {
			break
		}
	}
	return result
}

// lastReturnType extracts the final (non-function) return type from a curried
// function type chain.  E.g. for float -> float -> bool, returns "bool".
func (g *Generator) lastReturnType(t *ast.TFun) string {
	current := ast.Type(t.To)
	for {
		if fn, ok := current.(*ast.TFun); ok {
			current = fn.To
		} else {
			return g.typeToGo(current)
		}
	}
}

func (g *Generator) typeToGo(at ast.Type) string {
	if at == nil {
		return "struct{}"
	}
	switch t := at.(type) {
	case *ast.TIdent:
		switch t.Name {
		case "int":
			return "int"
		case "float":
			return "float64"
		case "bool":
			return "bool"
		case "string":
			return "string"
		case "unit":
			return "struct{}"
		case "list":
			return "list"
		case "option":
			return "option"
		case "result":
			return "result"
		case "owned_chan":
			return "owned_chan"
		default:
			return g.goName(t.Name)
		}
	case *ast.TApp:
		funcName := g.typeToGo(t.Func)
		argName := g.typeToGo(t.Arg)
		switch funcName {
		case "list":
			return "[]" + argName
		case "option":
			return "Option" + exported(argName)
		case "result":
			// result applied to a tuple → result<A, B>
			return "Result" + exported(argName)
		case "owned_chan":
			return "*C0Chan"
		default:
			return funcName + "_" + argName
		}
	case *ast.TFun:
		// Fully flatten curried function: float -> float -> bool → func(float64, float64) bool
		allParams := g.flattenFunFrom(t)
		finalReturn := g.lastReturnType(t)
		return fmt.Sprintf("func(%s) %s", strings.Join(allParams, ", "), finalReturn)
	case *ast.TTuple:
		if len(t.Elems) == 2 {
			return g.typeToGo(t.Elems[0]) + "_and_" + g.typeToGo(t.Elems[1])
		}
		parts := make([]string, len(t.Elems))
		for i, e := range t.Elems {
			parts[i] = g.typeToGo(e)
		}
		return strings.Join(parts, "_and_")
	case *ast.TVar:
		return "interface{}"
	case *ast.TRecord:
		return g.goName(g.recordNameFromType(t))
	case *ast.TChan:
		return "*C0Chan"
	case *ast.RefinementType:
		// Refinement types are transparent — only the inner type matters
		return g.typeToGo(t.Inner)
	default:
		return "interface{}"
	}
}

func (g *Generator) recordNameFromType(t *ast.TRecord) string {
	// Try to find which declared record type matches these fields
	for name, td := range g.records {
		if rk, ok := td.Kind.(*ast.RecordTypeKind); ok {
			if len(rk.Fields) == len(t.Fields) {
				match := true
				for i, f := range rk.Fields {
					if i >= len(t.Fields) || f.Name != t.Fields[i].Name {
						match = false
						break
					}
				}
				if match {
					return name
				}
			}
		}
	}
	return "Record"
}

// ---------------------------------------------------------------------------
// Source map: //line directives
// ---------------------------------------------------------------------------

func (g *Generator) emitLine(line, col int) {
	if g.srcFile != "" {
		g.emitf("//line %s:%d:%d\n", g.srcFile, line, col)
	}
	// After a //line directive, the next Go line maps to the given C0 line.
	// Record this remapping so the source map reflects it.
	g.recordMapping(line, col)
}

// ---------------------------------------------------------------------------
// Output helpers — all buffer writes go through writeStr for position tracking
// ---------------------------------------------------------------------------

// writeStr appends s to the output buffer, updating the Go line/column tracker.
func (g *Generator) writeStr(s string) {
	for _, r := range s {
		if r == '\n' {
			g.goLine++
			g.goCol = 1
		} else {
			g.goCol++
		}
	}
	g.buf.WriteString(s)
}

func (g *Generator) emitf(format string, args ...any) {
	s := g.indentStr() + fmt.Sprintf(format, args...)
	g.writeStr(s)
}

func (g *Generator) emit(s string) {
	g.writeStr(g.indentStr() + s)
}

func (g *Generator) indentStr() string {
	return strings.Repeat("\t", g.indent)
}

// ---------------------------------------------------------------------------
// Imports
// ---------------------------------------------------------------------------

// resolveOpens resolves all `open` statements in the module to Go import paths.
func (g *Generator) resolveOpens(mod *ast.Module) {
	if g.cfg == nil {
		return
	}
	for _, o := range mod.Opens {
		goPath, goPkg := g.cfg.ResolveImport(o.Path)
		// Skip self-imports (same package as current module)
		if goPkg == g.goPkg {
			continue
		}
		g.resolvedImports[o.Path] = goPath
		g.importPkgs[goPath] = goPkg
	}
}

// collectExterns gathers all extern import paths and registers name mappings.
func (g *Generator) collectExterns(mod *ast.Module) {
	for _, d := range mod.Decls {
		ed, ok := d.(*ast.ExternDecl)
		if !ok {
			continue
		}
		if ed.Lang != "go" {
			continue
		}
		pkgName := packageNameFromPath2(ed.Path)
		if ed.Path != "" {
			g.externImports[ed.Path] = pkgName
		}

		for _, ev := range ed.Vals {
			var qualified string
			if ed.Path == "" {
				qualified = ev.Name // same package, no prefix
			} else {
				qualified = pkgName + "." + ev.Name
			}
			g.externNames[ev.Name] = qualified
		}
	}
}

func (g *Generator) emitImports() {
	imports := make(map[string]string) // path → alias
	if g.needFmt {
		imports["fmt"] = "fmt"
	}
	// Add resolved imports from open statements
	for _, goPath := range g.resolvedImports {
		pkg := g.importPkgs[goPath]
		// If the Go package name matches the path's last segment, no alias needed
		if packageNameFromPath2(goPath) == pkg {
			imports[goPath] = pkg
		} else {
			imports[goPath] = pkg
		}
	}
	// Add extern imports
	for path, pkg := range g.externImports {
		imports[path] = pkg
	}
	// Add prelude imports (strconv, etc.)
	for path, pkg := range g.importPkgs {
		if path != "fmt" && path != "" {
			imports[path] = pkg
		}
	}
	if len(imports) == 0 {
		return
	}
	if len(imports) == 1 {
		for path := range imports {
			g.emitf("import %q\n\n", path)
		}
		return
	}
	g.emitf("import (\n")
	g.indent++
	for path := range imports {
		g.emitf("%q\n", path)
	}
	g.indent--
	g.emitf(")\n\n")
}

// packageNameFromPath2 extracts the last segment of a Go import path.
func packageNameFromPath2(path string) string {
	segments := strings.Split(path, "/")
	return segments[len(segments)-1]
}

// ---------------------------------------------------------------------------
// Tuple type generation
// ---------------------------------------------------------------------------

func (g *Generator) emitTupleTypes() {
	for name, fieldTypes := range g.usedTuple {
		// Generate struct for each unique tuple type
		g.emitf("type %s struct {\n", name)
		g.indent++
		for i, ft := range fieldTypes {
			g.emitf("F%d %s\n", i, ft)
		}
		g.indent--
		g.emitf("}\n\n")
	}
}

// ---------------------------------------------------------------------------
// Option type generation
// ---------------------------------------------------------------------------

func (g *Generator) emitOptionTypes() {
	for goType, elemGo := range g.usedOption {
		short := strings.TrimPrefix(goType, "Option")
		tagType := "OptionTag" + short
		g.emitf("// Option type: %s\n", goType)
		g.emitf("type %s byte\n\n", tagType)
		g.emitf("const (\n")
		g.indent++
		g.emitf("%sNone %s = iota\n", unexported(tagType), tagType)
		g.emitf("%sSome\n", unexported(tagType))
		g.indent--
		g.emitf(")\n\n")

		g.emitf("type %s struct {\n", goType)
		g.indent++
		g.emitf("tag  %s\n", tagType)
		g.emitf("some %s\n", elemGo)
		g.indent--
		g.emitf("}\n\n")

		g.emitf("func New%sNone() %s {\n", goType, goType)
		g.indent++
		g.emitf("return %s{tag: %sNone}\n", goType, unexported(tagType))
		g.indent--
		g.emitf("}\n\n")

		g.emitf("func New%sSome(v %s) %s {\n", goType, elemGo, goType)
		g.indent++
		g.emitf("return %s{tag: %sSome, some: v}\n", goType, unexported(tagType))
		g.indent--
		g.emitf("}\n\n")

		g.emitf("func (o %s) IsSome() bool { return o.tag == %sSome }\n\n", goType, unexported(tagType))
		g.emitf("func (o %s) MustSome() %s {\n", goType, elemGo)
		g.indent++
		g.emitf("if o.tag != %sSome { panic(\"Option.Some on None\") }\n", unexported(tagType))
		g.emitf("return o.some\n")
		g.indent--
		g.emitf("}\n\n")
		g.emitf("func (o %s) SomeOr(def %s) %s {\n", goType, elemGo, elemGo)
		g.indent++
		g.emitf("if o.tag == %sSome { return o.some }\n", unexported(tagType))
		g.emitf("return def\n")
		g.indent--
		g.emitf("}\n\n")
	}
}

// ---------------------------------------------------------------------------
// Result type generation
// ---------------------------------------------------------------------------

func (g *Generator) emitResultTypes() {
	for goType, parts := range g.usedResult {
		short := strings.TrimPrefix(goType, "Result")
		tagType := "ResultTag" + short

		okType, errType := "interface{}", "interface{}"
		if len(parts) >= 2 {
			okType = parts[0]
			errType = parts[1]
		}

		g.emitf("// Result type: %s\n", goType)
		g.emitf("type %s byte\n\n", tagType)
		g.emitf("const (\n")
		g.indent++
		g.emitf("%sOk %s = iota\n", unexported(tagType), tagType)
		g.emitf("%sErr\n", unexported(tagType))
		g.indent--
		g.emitf(")\n\n")

		g.emitf("type %s struct {\n", goType)
		g.indent++
		g.emitf("tag  %s\n", tagType)
		g.emitf("ok   %s\n", okType)
		g.emitf("err  %s\n", errType)
		g.indent--
		g.emitf("}\n\n")

		g.emitf("func New%sOk(v %s) %s {\n", goType, okType, goType)
		g.indent++
		g.emitf("return %s{tag: %sOk, ok: v}\n", goType, unexported(tagType))
		g.indent--
		g.emitf("}\n\n")

		g.emitf("func New%sErr(e %s) %s {\n", goType, errType, goType)
		g.indent++
		g.emitf("return %s{tag: %sErr, err: e}\n", goType, unexported(tagType))
		g.indent--
		g.emitf("}\n\n")

		g.emitf("func (r %s) IsOk() bool { return r.tag == %sOk }\n\n", goType, unexported(tagType))
		g.emitf("func (r %s) MustOk() %s {\n", goType, okType)
		g.indent++
		g.emitf("if r.tag != %sOk { panic(\"Result.Ok on Err\") }\n", unexported(tagType))
		g.emitf("return r.ok\n")
		g.indent--
		g.emitf("}\n\n")
		g.emitf("func (r %s) MustErr() %s {\n", goType, errType)
		g.indent++
		g.emitf("if r.tag != %sErr { panic(\"Result.Err on Ok\") }\n", unexported(tagType))
		g.emitf("return r.err\n")
		g.indent--
		g.emitf("}\n\n")
		g.emitf("func (r %s) OkOr(def %s) %s {\n", goType, okType, okType)
		g.indent++
		g.emitf("if r.tag == %sOk { return r.ok }\n", unexported(tagType))
		g.emitf("return def\n")
		g.indent--
		g.emitf("}\n\n")
	}
}

// ---------------------------------------------------------------------------
// Record type generation
// ---------------------------------------------------------------------------

func (g *Generator) emitRecordTypes() {
	for _, td := range g.records {
		rk, ok := td.Kind.(*ast.RecordTypeKind)
		if !ok {
			continue
		}
		goName := g.goName(td.Name)
		g.emitf("type %s struct {\n", goName)
		g.indent++
		for _, f := range rk.Fields {
			g.emitf("%s %s\n", exported(f.Name), g.typeToGo(f.Type))
		}
		g.indent--
		g.emitf("}\n\n")
	}
}

// ---------------------------------------------------------------------------
// ADT type generation
// ---------------------------------------------------------------------------

func (g *Generator) emitADTTypes() {
	for _, td := range g.adts {
		ak, ok := td.Kind.(*ast.ADTTypeKind)
		if !ok {
			continue
		}
		goName := g.goName(td.Name)
		iface := "is" + goName

		// Interface
		g.emitf("type %s interface {\n", goName)
		g.indent++
		g.emitf("%s()\n", iface)
		g.indent--
		g.emitf("}\n\n")

		// One struct per variant
		for _, c := range ak.Cases {
			varName := goName + exported(c.Name)
			g.emitf("type %s struct {\n", varName)
			g.indent++
			if c.Arg != nil {
				g.emitVariantFields(c.Arg)
			}
			g.indent--
			g.emitf("}\n\n")

			g.emitf("func (%s) %s() {}\n\n", varName, iface)

			// Constructor
			g.emitConstructorFunc(goName, varName, c)
		}
	}
}

func (g *Generator) emitVariantFields(arg ast.Type) {
	switch t := arg.(type) {
	case *ast.TRecord:
		for _, f := range t.Fields {
			g.emitf("%s %s\n", exported(f.Name), g.typeToGo(f.Type))
		}
	case *ast.TIdent:
		g.emitf("Value %s\n", g.typeToGo(t))
	case *ast.TTuple:
		for i, e := range t.Elems {
			g.emitf("F%d %s\n", i, g.typeToGo(e))
		}
	default:
		g.emitf("Value interface{}\n")
	}
}

// ---------------------------------------------------------------------------
// Opaque type generation (linear types — erased in Go)
// ---------------------------------------------------------------------------

func (g *Generator) emitOpaqueTypes() {
	for _, d := range g.opaqueTypes {
		goName := g.goName(d.Name)
		// Opaque linear types are erased — generate a placeholder type
		// alias that the Go type system will accept.
		g.emitf("type %s = interface{}\n\n", goName)
	}
}

func (g *Generator) emitConstructorFunc(adtGoName, varName string, c ast.ADTCase) {
	g.emitf("func New%s", varName)
	// Parameters
	hasParams := false
	if c.Arg != nil {
		params := g.variantFieldParams(c.Arg)
		if len(params) > 0 {
			g.buf.WriteString("(")
			hasParams = true
			for i, p := range params {
				if i > 0 {
					g.buf.WriteString(", ")
				}
				g.buf.WriteString(p.name + " " + p.goType)
			}
			g.buf.WriteString(")")
		}
	}
	if !hasParams {
		g.buf.WriteString("()")
	}
	g.buf.WriteString(" " + adtGoName + " {\n")
	g.indent++
	g.emitf("return %s{", varName)
	if c.Arg != nil {
		args := g.variantFieldArgs(c.Arg)
		for i, a := range args {
			if i > 0 {
				g.buf.WriteString(", ")
			}
			g.buf.WriteString(a.field + ": " + a.param)
		}
	}
	g.buf.WriteString("}\n")
	g.indent--
	g.emitf("}\n\n")
}

type fieldParam struct {
	name, goType, field, param string
}

func (g *Generator) variantFieldParams(arg ast.Type) []fieldParam {
	var result []fieldParam
	switch t := arg.(type) {
	case *ast.TRecord:
		for _, f := range t.Fields {
			result = append(result, fieldParam{
				name:   unexported(f.Name),
				goType: g.typeToGo(f.Type),
				field:  exported(f.Name),
				param:  unexported(f.Name),
			})
		}
	case *ast.TIdent:
		result = append(result, fieldParam{
			name:   "v",
			goType: g.typeToGo(t),
			field:  "Value",
			param:  "v",
		})
	case *ast.TTuple:
		for i, e := range t.Elems {
			n := fmt.Sprintf("f%d", i)
			result = append(result, fieldParam{
				name:   n,
				goType: g.typeToGo(e),
				field:  fmt.Sprintf("F%d", i),
				param:  n,
			})
		}
	}
	return result
}

func (g *Generator) variantFieldArgs(arg ast.Type) []fieldParam {
	return g.variantFieldParams(arg)
}

func (g *Generator) variantFieldName(arg ast.Type, adtName, caseName string) string {
	if rt, ok := arg.(*ast.TRecord); ok {
		// For records, emitVariantFields expands fields directly into the struct.
		// If the record matches a known type, use that name.
		// Otherwise, use empty string (fields are directly accessible).
		name := g.recordNameFromType(rt)
		if name != "Record" {
			return name
		}
		// Inline record: fields are expanded directly — no wrapper field name.
		return ""
	}
	// For non-record payloads, the field is always "Value"
	return "Value"
}

// ---------------------------------------------------------------------------
// Top-level let declarations
// ---------------------------------------------------------------------------

// patternVarName returns the variable name bound by a simple pattern.
func (g *Generator) patternVarName(p ast.Pattern) string {
	if ip, ok := p.(*ast.IdentPattern); ok {
		return ip.Name
	}
	return "_"
}

func (g *Generator) emitLetDecl(d *ast.LetDecl) {
	// Active patterns get a special function name
	if d.ActivePattern {
		for _, b := range d.Bindings {
			goFuncName := "__active_" + b.Name
			g.currentFunc = b.Name
			// Emit as a regular function with the active-pattern name
			g.emitf("func %s", goFuncName)
			g.emitParams(b.Params)
			// Return type is always an option type
			retGo := "interface{}"
			if b.RetType != nil {
				retGo = g.typeToGo(b.RetType)
			}
			g.buf.WriteString(" " + retGo)
			g.buf.WriteString(" {\n")
			g.indent++
			if retGo != "struct{}" {
				g.emitReturnExpr(b.Body)
			} else {
				g.emitExpr(b.Body, true)
			}
			g.indent--
			g.emitf("}\n\n")
		}
		return
	}

	for _, b := range d.Bindings {
		g.currentFunc = b.Name
		funcName := b.Name
		// Keep `main` as `main` for Go entry point (handled by being lowercase)
		_ = funcName

		// Record source mapping: we don't have the AST source location directly,
		// so we use the current Go output position and approximate C0 line from
		// the function name context.
		g.recordMapping(g.goLine, 0) // approximate C0 line = current Go line

		// Filter out empty-named params (from `()` parsing quirk)
		realParams := make([]ast.Param, 0, len(b.Params))
		for _, p := range b.Params {
			if p.Name != "" {
				realParams = append(realParams, p)
			}
		}

		if len(realParams) == 0 && !isCompoundExpr(b.Body) {
			// Simple value binding
			if d.Mutable {
				g.emitf("var %s = ", funcName)
			} else {
				g.emitf("var %s = ", funcName)
			}
			g.emitExpr(b.Body, false)
			g.buf.WriteString("\n\n")
		} else if len(realParams) == 0 && g.isChanMakeExpr(b.Body) {
			// let ch = Chan.make () → ch := C0ChanMake()
			g.usedChan = true
			g.emitf("%s := C0ChanMake()\n", funcName)
		} else if len(realParams) == 0 {
			// Compound value binding: emit as a func that returns the value
			g.emitf("func %s()", funcName)
			retType := ""
			if b.RetType != nil {
				retType = g.typeToGo(b.RetType)
			}
			if retType != "" && retType != "struct{}" {
				g.buf.WriteString(" " + retType)
			}
			g.buf.WriteString(" {\n")
			g.indent++
			if retType != "" && retType != "struct{}" {
				g.emitReturnExpr(b.Body)
			} else {
				g.emitExpr(b.Body, true)
			}
			g.indent--
			g.emitf("}\n\n")
		} else {
			// Function
			g.emitf("func %s", funcName)
			g.emitParams(realParams)
			hasReturn := b.RetType != nil && g.typeToGo(b.RetType) != "struct{}"

			// Check for return type refinement → use named return value
			hasPostcondition := false
			var postRT *ast.RefinementType
			if hasReturn {
				if rt, ok := b.RetType.(*ast.RefinementType); ok {
					hasPostcondition = true
					postRT = rt
					g.buf.WriteString(" (result " + g.typeToGo(b.RetType) + ")")
				} else {
					g.buf.WriteString(" " + g.typeToGo(b.RetType))
				}
			}

			g.buf.WriteString(" {\n")
			g.indent++

			// Emit postcondition check via defer
			if hasPostcondition {
				g.emitPostconditionCheck(funcName, postRT)
			}

			// Record refinement-annotated parameters and emit precondition checks.
			// Proven call sites can skip checks (checked via g.provenSites lookup),
			// but the in-body check serves as a safety net for all callers.
			var refined []refinedParam
			for i, p := range realParams {
				if rt, ok := p.Type.(*ast.RefinementType); ok {
					refined = append(refined, refinedParam{index: i, name: p.Name, rt: rt})
				}
			}
			if len(refined) > 0 {
				g.refinementParams[funcName] = refined
			}

			// Emit precondition checks for each parameter (runtime safety net)
			for _, rp := range refined {
				g.emitPreconditionCheck(funcName, rp.name, rp.rt)
			}

			if hasReturn {
				g.emitReturnExpr(b.Body)
			} else {
				g.emitExpr(b.Body, true)
			}
			g.indent--
			g.emitf("}\n\n")
		}
		g.currentFunc = ""
	}
}

func isCompoundExpr(e ast.Expr) bool {
	switch e.(type) {
	case *ast.LitExpr, *ast.IdentExpr:
		return false
	}
	return true
}

// isChanMakeExpr checks whether an expression is a Chan.make () or OwnedChan.make () call.
func (g *Generator) isChanMakeExpr(e ast.Expr) bool {
	app, ok := e.(*ast.AppExpr)
	if !ok {
		return false
	}
	field, ok := app.Func.(*ast.FieldAccessExpr)
	if !ok {
		return false
	}
	qualified := g.fieldAccessName(field)
	return qualified == "Chan.make" || qualified == "OwnedChan.make"
}

func (g *Generator) emitParams(params []ast.Param) {
	g.buf.WriteString("(")
	first := true
	for _, p := range params {
		// Expand open record params into individual field params
		if p.Type != nil {
			if rt, ok := p.Type.(*ast.TRecord); ok && rt.Open {
				for _, f := range rt.Fields {
					if !first {
						g.buf.WriteString(", ")
					}
					first = false
					g.buf.WriteString(p.Name + "_" + f.Name)
					g.buf.WriteString(" ")
					g.buf.WriteString(g.typeToGo(f.Type))
				}
				continue
			}
		}
		if !first {
			g.buf.WriteString(", ")
		}
		first = false
		g.buf.WriteString(p.Name)
		g.buf.WriteString(" ")
		if p.Type != nil {
			g.buf.WriteString(g.typeToGo(p.Type))
		} else {
			g.buf.WriteString("interface{}")
		}
	}
	g.buf.WriteString(")")
}

// ---------------------------------------------------------------------------
// Expression emission
// ---------------------------------------------------------------------------

func (g *Generator) emitExpr(e ast.Expr, isStmt bool) {
	switch e := e.(type) {
	case *ast.LitExpr:
		g.emitLit(e)
	case *ast.IdentExpr:
		g.emitIdent(e)
	case *ast.ConstructorExpr:
		g.emitConstructor(e)
	case *ast.AppExpr:
		g.emitApp(e, isStmt)
	case *ast.IfExpr:
		g.emitIf(e)
	case *ast.MatchExpr:
		g.emitMatch(e)
	case *ast.LetInExpr:
		g.emitLetIn(e)
	case *ast.FunExpr:
		g.emitFun(e)
	case *ast.BinaryExpr:
		g.emitBinary(e)
	case *ast.PipeExpr:
		g.emitPipe(e)
	case *ast.QuestionExpr:
		g.emitQuestion(e)
	case *ast.RecordExpr:
		g.emitRecord(e)
	case *ast.RecordUpdateExpr:
		g.emitRecordUpdate(e)
	case *ast.FieldAccessExpr:
		g.emitFieldAccess(e)
	case *ast.TupleExpr:
		g.emitTuple(e)
	case *ast.ListExpr:
		g.emitList(e)
	case *ast.ParenExpr:
		g.emitExpr(e.Inner, isStmt)
	case *ast.IsExpr:
		g.emitIsExpr(e)
	case *ast.AsMatchExpr:
		g.emitAsMatchExpr(e)
	case *ast.GoExpr:
		g.emitGoExpr(e)
	case *ast.SelectExpr:
		g.emitSelectExpr(e)
	case *ast.UsingExpr:
		g.emitUsing(e, isStmt)
	case *ast.RegionExpr:
		g.emitRegion(e)
	default:
		g.buf.WriteString("/* TODO: " + fmt.Sprintf("%T", e) + " */")
	}
}

func (g *Generator) emitLit(e *ast.LitExpr) {
	switch e.Kind {
	case token.INT:
		g.buf.WriteString(fmt.Sprintf("%v", e.Value))
	case token.FLOAT:
		// Use a format that Go sees as float (add .0 for whole numbers)
		s := fmt.Sprintf("%v", e.Value)
		if !strings.Contains(s, ".") && !strings.Contains(s, "e") && !strings.Contains(s, "E") {
			s += ".0"
		}
		g.buf.WriteString(s)
	case token.STRING:
		g.buf.WriteString(fmt.Sprintf("%q", e.Value))
	case token.TRUE:
		g.buf.WriteString("true")
	case token.FALSE:
		g.buf.WriteString("false")
	case token.UNIT:
		g.buf.WriteString("struct{}{}")
	}
}

func (g *Generator) emitIdent(e *ast.IdentExpr) {
	// Check for extern-qualified names
	if qualified, ok := g.externNames[e.Name]; ok {
		g.buf.WriteString(qualified)
		return
	}
	// Use the name as-is; prelude lowerings are handled in emitApp
	g.buf.WriteString(e.Name)
}

func (g *Generator) emitConstructor(e *ast.ConstructorExpr) {
	// Check for extern-qualified names first
	if qualified, ok := g.externNames[e.Name]; ok {
		g.buf.WriteString(qualified)
		if e.Arg != nil {
			g.buf.WriteString("(")
			g.emitExpr(e.Arg, false)
			g.buf.WriteString(")")
		}
		return
	}
	// Check user-defined ADTs first (they take priority over built-in names)
	varName := g.findVariantStruct(e.Name)
	if varName != "" {
		g.buf.WriteString("New" + varName)
		g.buf.WriteString("(")
		if e.Arg != nil {
			if rec, ok := e.Arg.(*ast.RecordExpr); ok && g.isVariantRecordPayload(e.Name) {
				// Inline record fields — expand as individual constructor args
				for i, f := range rec.Fields {
					if i > 0 {
						g.buf.WriteString(", ")
					}
					g.emitExpr(f.Value, false)
				}
			} else {
				g.emitExpr(e.Arg, false)
			}
		}
		g.buf.WriteString(")")
		return
	}

	switch e.Name {
	case "None":
		optType := g.findOptionType()
		g.buf.WriteString("New" + optType + "None()")
		return
	case "Some":
		optType := g.findOptionType()
		g.buf.WriteString("New" + optType + "Some(")
		g.emitExpr(e.Arg, false)
		g.buf.WriteString(")")
		return
	case "Ok":
		resType := g.findResultType()
		g.buf.WriteString("New" + resType + "Ok(")
		g.emitExpr(e.Arg, false)
		g.buf.WriteString(")")
		return
	case "Error":
		resType := g.findResultType()
		g.buf.WriteString("New" + resType + "Err(")
		g.emitExpr(e.Arg, false)
		g.buf.WriteString(")")
		return
	default:
		// Unknown constructor — emit as plain call
		g.buf.WriteString(e.Name)
		if e.Arg != nil {
			g.buf.WriteString("(")
			g.emitExpr(e.Arg, false)
			g.buf.WriteString(")")
		}
	}
}

func (g *Generator) isListMatch(e *ast.MatchExpr) bool {
	for _, arm := range e.Arms {
		switch arm.Pattern.(type) {
		case *ast.ListPattern, *ast.ConsPattern:
			return true
		}
	}
	return false
}

func (g *Generator) emitListMatch(e *ast.MatchExpr) {
	// Emit as if/else chain: if len == 0 { ... } else { first := xs[0]; ... }
	g.varCounter++
	tmp := fmt.Sprintf("_l%d", g.varCounter)
	g.emitf("%s := ", tmp)
	g.emitExpr(e.Scrutinee, false)
	g.buf.WriteString("\n")

	for i, arm := range e.Arms {
		prefix := "if "
		if i > 0 {
			prefix = "} else if "
		}
		switch p := arm.Pattern.(type) {
		case *ast.ListPattern:
			if len(p.Elems) == 0 {
				g.emitf("%slen(%s) == 0 {\n", prefix, tmp)
				g.indent++
				g.emitReturnExpr(arm.Body)
				g.indent--
			}
		case *ast.ConsPattern:
			g.emitf("%slen(%s) > 0 {\n", prefix, tmp)
			g.indent++
			if _, isWild := p.Head.(*ast.WildcardPattern); !isWild {
				g.emitf("%s := %s[0]\n", g.patternVarName(p.Head), tmp)
			}
			if _, isWild := p.Tail.(*ast.WildcardPattern); !isWild {
				g.emitf("%s := %s[1:]\n", g.patternVarName(p.Tail), tmp)
			}
			g.emitReturnExpr(arm.Body)
			g.indent--
		case *ast.WildcardPattern, *ast.IdentPattern:
			if prefix == "if " {
				g.emitf("{\n")
			} else {
				g.emitf("} else {\n")
			}
			g.indent++
			if ip, ok := p.(*ast.IdentPattern); ok {
				g.emitf("%s := %s\n", ip.Name, tmp)
			}
			g.emitReturnExpr(arm.Body)
			g.indent--
			g.emitf("}\n")
		}
	}
	// Close the final if/else block and add a fallback panic for Go's exhaustiveness check
	g.emitf("}\n")
	g.emitf("panic(\"unreachable: unhandled list pattern\")\n")
}

func (g *Generator) isResultMatch(e *ast.MatchExpr) bool {
	// Check if all patterns are Ok/Error constructors (result type match)
	if len(e.Arms) == 0 {
		return false
	}
	for _, arm := range e.Arms {
		cp, ok := arm.Pattern.(*ast.ConstructorPattern)
		if !ok {
			return false
		}
		if cp.Name != "Ok" && cp.Name != "Error" {
			return false
		}
	}
	return true
}

func (g *Generator) emitResultMatch(e *ast.MatchExpr) {
	// Emit as: tmp := scrutinee; if tmp.IsOk() { <Ok arms> } else { <Error arms> }
	g.varCounter++
	tmp := fmt.Sprintf("_tmp%d", g.varCounter)
	g.emitf("%s := ", tmp)
	g.emitExpr(e.Scrutinee, false)
	g.buf.WriteString("\n")

	// Group Ok and Error arms
	var okArms, errArms []ast.MatchArm
	for _, arm := range e.Arms {
		cp := arm.Pattern.(*ast.ConstructorPattern)
		switch cp.Name {
		case "Ok":
			okArms = append(okArms, arm)
		case "Error":
			errArms = append(errArms, arm)
		}
	}

	if len(okArms) > 0 {
		g.emitf("if %s.IsOk() {\n", tmp)
		g.indent++
		g.emitResultArms(okArms, tmp)
		g.indent--
		if len(errArms) > 0 {
			g.emitf("} else {\n")
			g.indent++
			g.emitResultArms(errArms, tmp)
			g.indent--
		}
		g.emitf("}\n")
	} else if len(errArms) > 0 {
		g.emitf("if !%s.IsOk() {\n", tmp)
		g.indent++
		g.emitResultArms(errArms, tmp)
		g.indent--
		g.emitf("}\n")
	}
}

func (g *Generator) emitResultArms(arms []ast.MatchArm, tmp string) {
	armKind := "Ok"
	if len(arms) > 0 {
		if cp, ok := arms[0].Pattern.(*ast.ConstructorPattern); ok {
			armKind = cp.Name
		}
	}
	extractor := tmp + ".MustOk()"
	if armKind == "Error" {
		extractor = tmp + ".MustErr()"
	}

	// If only one arm with a simple pattern (no nested constructor), emit directly
	if len(arms) == 1 {
		cp := arms[0].Pattern.(*ast.ConstructorPattern)
		if cp.Arg != nil && isSimplePattern(cp.Arg) {
			g.emitPatternBinding(cp.Arg, extractor)
		} else if cp.Arg != nil {
			// Nested constructor pattern: emit as type switch
			g.emitResultNestedArms(arms, extractor)
			return
		}
		g.emitReturnExpr(arms[0].Body)
		return
	}

	// Multiple arms
	g.emitResultNestedArms(arms, extractor)
}

func isSimplePattern(p ast.Pattern) bool {
	switch p.(type) {
	case *ast.IdentPattern, *ast.WildcardPattern:
		return true
	}
	return false
}

func (g *Generator) emitResultNestedArms(arms []ast.MatchArm, prefix string) {
	// Emit as type switch on the extracted value
	g.varCounter++
	ev := fmt.Sprintf("_e%d", g.varCounter)
	g.emitf("%s := %s\n", ev, prefix)
	g.emitf("switch %s := %s.(type) {\n", ev, ev)
	g.indent++
	for _, arm := range arms {
		cp := arm.Pattern.(*ast.ConstructorPattern)
		// The inner pattern determines the variant struct name
		innerName := ""
		if cp.Arg != nil {
			if icp, ok := cp.Arg.(*ast.ConstructorPattern); ok {
				innerName = icp.Name
			}
		}
		structName := g.findVariantStruct(innerName)
		if structName == "" {
			structName = g.goName(innerName)
		}
		if structName == "" {
			structName = cp.Name
		}
		g.emitf("case %s:\n", structName)
		g.indent++
		if cp.Arg != nil {
			g.emitPatternBinding(cp.Arg, ev)
		}
		g.emitReturnExpr(arm.Body)
		g.indent--
	}
	g.emitf("default:\n")
	g.indent++
	g.emitf("panic(\"unreachable: unhandled result variant\")\n")
	g.indent--
	g.indent--
	g.emitf("}\n")
}

func (g *Generator) findOptionType() string {
	if ret, ok := g.funcRetType[g.currentFunc]; ok && strings.HasPrefix(ret, "Option") {
		return ret
	}
	return "OptionT"
}

func (g *Generator) findResultType() string {
	if ret, ok := g.funcRetType[g.currentFunc]; ok && strings.HasPrefix(ret, "Result") {
		return ret
	}
	return "ResultT"
}

func (g *Generator) emitApp(e *ast.AppExpr, isStmt bool) {
	// Function application
	// Check if this is a method-like call on a constructor (e.g., Console.print_line)
	if field, ok := e.Func.(*ast.FieldAccessExpr); ok {
		// Check prelude for qualified names like Console.print_line
		qualifiedName := g.fieldAccessName(field)
		if qualifiedName != "" {
			if b := g.prelude.Lookup(qualifiedName); b != nil {
				g.emitPreludeCall(b, []ast.Expr{e.Arg}, e)
				if isStmt {
					g.buf.WriteString("\n")
				}
				return
			}
		}
		// qualified call like Console.print_line
		g.emitFieldAccess(field)
		g.buf.WriteString("(")
		g.emitExpr(e.Arg, false)
		g.buf.WriteString(")")
		if isStmt {
			g.buf.WriteString("\n")
		}
		return
	}

	// Regular function application: f(x, y, ...)
	// For curried calls, collect all arguments
	args := g.collectArgs(e)

	funcExpr := args[0].(ast.Expr)
	args = args[1:]

	// Flatten ConstructorExpr with embedded Arg for extern functions.
	// The parser may represent "Mod a b" as
	// AppExpr(Func=ConstructorExpr(Mod, Arg=a), Arg=b), which means
	// collectArgs only sees [ConstructorExpr(Mod, a), b] instead of
	// [Mod, a, b]. Un-embed the first arg so all args are in one flat list.
	if cons, ok := funcExpr.(*ast.ConstructorExpr); ok && cons.Arg != nil {
		if _, isExtern := g.externNames[cons.Name]; isExtern {
			funcExpr = &ast.IdentExpr{Name: cons.Name}
			args = append([]ast.Expr{cons.Arg}, args...)
		}
	}

	// Check for prelude lowering before partial application/user function
	funcName := g.getExprName(funcExpr)
	if funcName == "" {
		// Also recognize qualified names like Chan.send as prelude calls
		if field, ok := funcExpr.(*ast.FieldAccessExpr); ok {
			funcName = g.fieldAccessName(field)
		}
	}
	if b := g.prelude.Lookup(funcName); b != nil {
		g.emitPreludeCall(b, args, e)
		if isStmt {
			g.buf.WriteString("\n")
		}
		return
	}

	// Check for partial application
	if funcName != "" {
		paramCount := g.funcParamCount[funcName]
		if paramCount > 0 && len(args) < paramCount {
			// Partial application: emit closure
			g.emitPartialApp(funcName, args, paramCount)
			return
		}
	}

	// Expand row-polymorphic arguments: for each arg that corresponds to
	// a row param, extract individual fields from the record literal.
	if funcName != "" && len(g.rowParams[funcName]) > 0 {
		g.emitExpr(funcExpr, false)
		g.buf.WriteString("(")
		rowFields := g.rowParams[funcName]
		first := true
		for i, arg := range args {
			if i >= len(rowFields) {
				// Extra args after row param
				if !first {
					g.buf.WriteString(", ")
				}
				first = false
				g.emitExpr(arg, false)
				continue
			}
			// Row param arg: extract fields from the record expression
			if rec, ok := arg.(*ast.RecordExpr); ok {
				for _, rf := range rowFields {
					if !first {
						g.buf.WriteString(", ")
					}
					first = false
					found := false
					for _, f := range rec.Fields {
						if f.Name == rf {
							if f.Value != nil {
								g.emitExpr(f.Value, false)
							} else {
								g.buf.WriteString(f.Name)
							}
							found = true
							break
						}
					}
					if !found {
						g.buf.WriteString("/* missing: " + rf + " */")
					}
				}
			} else {
				// Non-record arg for row param — emit as-is
				if !first {
					g.buf.WriteString(", ")
				}
				first = false
				g.emitExpr(arg, false)
			}
		}
		g.buf.WriteString(")")
		if isStmt {
			g.buf.WriteString("\n")
		}
		return
	}

	g.emitExpr(funcExpr, false)
	g.buf.WriteString("(")
	// When the function has zero Go params (because its only param was unit `()`),
	// skip emitting arguments — don't emit `struct{}{}`.
	if funcName != "" {
		if paramCnt, ok := g.funcParamCount[funcName]; ok && paramCnt == 0 {
			goto afterArgs
		}
		// For extern functions, strip unit arguments — C0's `unit` type maps
		// to zero Go parameters.
		if _, isExtern := g.externNames[funcName]; isExtern {
			var filtered []ast.Expr
			for _, a := range args {
				// Unit literals are LitExpr with Kind == token.UNIT
				if lit, ok := a.(*ast.LitExpr); !ok || lit.Kind != token.UNIT {
					filtered = append(filtered, a)
				}
			}
			args = filtered
		}
	}
	for i, arg := range args {
		if i > 0 {
			g.buf.WriteString(", ")
		}
		g.emitExpr(arg.(ast.Expr), false)
	}
afterArgs:
	g.buf.WriteString(")")
	if isStmt {
		g.buf.WriteString("\n")
	}
}

func (g *Generator) getExprName(e ast.Expr) string {
	if ident, ok := e.(*ast.IdentExpr); ok {
		return ident.Name
	}
	return ""
}

func (g *Generator) emitPartialApp(funcName string, givenArgs []ast.Expr, totalParams int) {
	remaining := totalParams - len(givenArgs)
	goFuncName := funcName

	// Determine return type from function's return type (now correctly flattened)
	retType := "interface{}"
	if rt, ok := g.funcRetType[funcName]; ok {
		retType = rt
	}

	// Get the types of the remaining parameters
	paramTypes := g.funcParamTypes[funcName]
	remainingTypes := paramTypes[len(givenArgs):] // types of params not yet given

	g.buf.WriteString("func(")
	for i := 0; i < remaining; i++ {
		if i > 0 {
			g.buf.WriteString(", ")
		}
		g.buf.WriteString(fmt.Sprintf("_p%d", i+1))
		g.buf.WriteString(" ")
		// Use stored param type if available, otherwise interface{}
		if i < len(remainingTypes) {
			g.buf.WriteString(remainingTypes[i])
		} else {
			g.buf.WriteString("interface{}")
		}
	}
	g.buf.WriteString(") " + retType + " {\n")
	g.indent++
	g.emitf("return %s(", goFuncName)
	for i, arg := range givenArgs {
		if i > 0 {
			g.buf.WriteString(", ")
		}
		g.emitExpr(arg, false)
	}
	for i := 0; i < remaining; i++ {
		if len(givenArgs) > 0 || i > 0 {
			g.buf.WriteString(", ")
		}
		g.buf.WriteString(fmt.Sprintf("_p%d", i+1))
	}
	g.buf.WriteString(")\n")
	g.indent--
	g.emitf("}")
}

// emitPreludeCall emits Go code for a call to a prelude function.
func (g *Generator) emitPreludeCall(b *prelude.Binding, args []ast.Expr, callExpr ast.Expr) {
	l := b.Lowering

	// Custom lowering templates
	switch l.Custom {
	case "assert":
		// assert cond → if !cond { panic("assertion failed") }
		g.buf.WriteString("if !(")
		if len(args) > 0 {
			g.emitExpr(args[0], false)
		}
		g.buf.WriteString(") { panic(\"assertion failed\") }")
		return
	case "assert_equal":
		// assert_equal a b → if a != b { panic("assert_equal failed") }
		if len(args) >= 2 {
			g.buf.WriteString("if ")
			g.emitExpr(args[0], false)
			g.buf.WriteString(" != ")
			g.emitExpr(args[1], false)
			g.buf.WriteString(" { panic(\"assert_equal failed\") }")
		}
		return

	case "chan_make":
		// Chan.make () → C0ChanMake()
		g.usedChan = true
		g.buf.WriteString("C0ChanMake()")
		return

	case "chan_send":
		// Chan.send ch v → C0ChanSend(ch, v)
		g.usedChan = true
		g.buf.WriteString("C0ChanSend(")
		if len(args) >= 2 {
			g.emitExpr(args[0], false)
			g.buf.WriteString(", ")
			g.emitExpr(args[1], false)
		}
		g.buf.WriteString(")")
		return

	case "chan_recv":
		// Chan.recv ch → C0ChanRecv(ch).(T)
		g.usedChan = true
		g.buf.WriteString("C0ChanRecv(")
		if len(args) >= 1 {
			g.emitExpr(args[0], false)
		}
		g.buf.WriteString(")")
		// Add type assertion if the concrete element type is known.
		elemGo := g.resolveChanElemType(args)
		if elemGo == "" || elemGo == "interface{}" {
			// Fallback: the call expression's return type IS the element type
			if g.typeMap != nil && callExpr != nil {
				if retType, ok := g.typeMap[callExpr]; ok {
					elemGo = g.internalTypeToGo(retType)
				}
			}
		}
		if elemGo != "" && elemGo != "interface{}" {
			g.buf.WriteString(".(" + elemGo + ")")
		}
		return

	case "chan_close":
		// Chan.close ch → C0ChanClose(ch)
		g.usedChan = true
		g.buf.WriteString("C0ChanClose(")
		if len(args) >= 1 {
			g.emitExpr(args[0], false)
		}
		g.buf.WriteString(")")
		return

	case "owned_chan_make":
		// OwnedChan.make () → C0ChanMake()
		g.usedChan = true
		g.buf.WriteString("C0ChanMake()")
		return

	case "owned_chan_send":
		// OwnedChan.send ch v → C0ChanSend(ch, v)
		g.usedChan = true
		g.buf.WriteString("C0ChanSend(")
		if len(args) >= 2 {
			g.emitExpr(args[0], false)
			g.buf.WriteString(", ")
			g.emitExpr(args[1], false)
		}
		g.buf.WriteString(")")
		return

	case "owned_chan_recv":
		// OwnedChan.recv ch → C0ChanRecv(ch).(T)
		g.usedChan = true
		g.buf.WriteString("C0ChanRecv(")
		if len(args) >= 1 {
			g.emitExpr(args[0], false)
		}
		g.buf.WriteString(")")
		// Add type assertion if the concrete element type is known.
		elemGo := g.resolveChanElemType(args)
		if elemGo == "" || elemGo == "interface{}" {
			if g.typeMap != nil && callExpr != nil {
				if retType, ok := g.typeMap[callExpr]; ok {
					elemGo = g.internalTypeToGo(retType)
				}
			}
		}
		if elemGo != "" && elemGo != "interface{}" {
			g.buf.WriteString(".(" + elemGo + ")")
		}
		return

	case "owned_chan_close":
		// OwnedChan.close ch → C0ChanClose(ch)
		g.usedChan = true
		g.buf.WriteString("C0ChanClose(")
		if len(args) >= 1 {
			g.emitExpr(args[0], false)
		}
		g.buf.WriteString(")")
		return
	}

	// Operator lowering: e.g., string_concat → a + b
	if l.Operator != "" {
		if len(args) >= 2 {
			// Emit as binary operator between first two arguments
			g.emitExpr(args[0], false)
			g.buf.WriteString(" " + l.Operator + " ")
			g.emitExpr(args[1], false)
		} else {
			g.emitExpr(args[0], false)
		}
		return
	}

	// Standard function call lowering
	if l.Func != "" {
		// Add import if needed
		if l.Pkg != "" {
			g.needFmt = (l.Pkg == "fmt") || g.needFmt
			g.importPkgs[l.Pkg] = l.Pkg
		}

		// Handle wrapped calls (e.g., strconv.FormatFloat with Sprintf)
		if l.Wrap == "fmt.Sprintf" {
			g.buf.WriteString("fmt.Sprintf(\"%v\"")
			g.needFmt = true
			if len(args) > 0 {
				g.buf.WriteString(", ")
				g.emitExpr(args[0], false)
			}
			g.buf.WriteString(")")
			return
		}

		g.buf.WriteString(l.Func)
		g.buf.WriteString("(")
		for i, arg := range args {
			if i > 0 {
				g.buf.WriteString(", ")
			}
			g.emitExpr(arg, false)
		}
		g.buf.WriteString(")")
	}
}

func (g *Generator) collectArgs(e ast.Expr) []ast.Expr {
	var result []ast.Expr
	current := e
	for {
		if app, ok := current.(*ast.AppExpr); ok {
			result = append([]ast.Expr{app.Arg}, result...)
			current = app.Func
		} else {
			result = append([]ast.Expr{current}, result...)
			break
		}
	}
	return result
}

// isNotPattern checks if an IfExpr matches the desugaring of `not expr`:
//   if condition then false else true
func isNotPattern(e *ast.IfExpr) bool {
	falseLit, thenOk := e.ThenBranch.(*ast.LitExpr)
	trueLit, elseOk := e.ElseBranch.(*ast.LitExpr)
	return thenOk && elseOk &&
		falseLit.Kind == token.FALSE && falseLit.Value == false &&
		trueLit.Kind == token.TRUE && trueLit.Value == true
}

func (g *Generator) emitIf(e *ast.IfExpr) {
	// Detect the `not` desugar pattern: if condition then false else true → !condition
	if isNotPattern(e) {
		g.buf.WriteString("!(")
		g.emitExpr(e.Cond, false)
		g.buf.WriteString(")")
		return
	}

	// General case: emit as a Go if/else statement.
	// When used as an expression (in a function call), this is valid Go because
	// each branch terminates with `return` emitting `return false / return true`.
	g.emitf("if ")
	g.emitExpr(e.Cond, false)
	g.buf.WriteString(" {\n")
	g.indent++
	g.emitReturnExpr(e.ThenBranch)
	g.indent--
	g.emitf("} else {\n")
	g.indent++
	g.emitReturnExpr(e.ElseBranch)
	g.indent--
	g.emitf("}\n")
}

func (g *Generator) emitMatch(e *ast.MatchExpr) {
	// Record mapping for the match expression
	g.recordMapping(g.goLine, 0)

	// Check for active patterns
	if g.hasActivePattern(e) {
		g.emitActiveMatch(e)
		return
	}

	// Check for list patterns ([] and ::)
	if g.isListMatch(e) {
		g.emitListMatch(e)
		return
	}

	// Check if this is a result type match (concrete struct, not interface)
	if g.isResultMatch(e) {
		g.emitResultMatch(e)
		return
	}

	// Store scrutinee in a temp for use in pattern bindings
	g.varCounter++
	scrutVar := fmt.Sprintf("_s%d", g.varCounter)
	g.emitf("%s := ", scrutVar)
	g.emitExpr(e.Scrutinee, false)
	g.buf.WriteString("\n")

	g.emitf("switch v := %s.(type) {\n", scrutVar)
	g.indent++

	// Group consecutive arms by struct name to avoid duplicate case labels
	groups := groupMatchArms(e.Arms, g)
	hasDefault := false

	for _, grp := range groups {
		if grp.structName == "" || grp.structName == "default" {
			hasDefault = true
			g.emitf("default:\n")
			g.indent++
			for _, arm := range grp.arms {
				if ip, ok := arm.Pattern.(*ast.IdentPattern); ok {
					g.emitf("%s := %s\n", ip.Name, scrutVar)
				}
				if len(grp.arms) == 1 {
					g.emitReturnExpr(arm.Body)
				}
			}
			g.indent--
			continue
		}

		g.emitf("case %s:\n", grp.structName)
		g.indent++

		// Emit pattern bindings from the first arm (or _ = v to suppress unused warning)
		if cp, ok := grp.arms[0].Pattern.(*ast.ConstructorPattern); ok && cp.Arg != nil {
			// Use the variant's struct field name to access the payload (if not expanded inline)
			fieldName := g.adtVariantField(cp.Name)
			prefix := "v"
			if fieldName != "" {
				prefix = "v." + fieldName
			}
			g.emitPatternBinding(cp.Arg, prefix)
		} else {
			g.emitf("_ = v\n")
		}

		// Emit bodies as if/else chain
		for i, arm := range grp.arms {
			if i > 0 {
				g.emitf("} else ")
			}
			if arm.Guard != nil {
				if i > 0 {
					g.buf.WriteString("if ")
				} else {
					g.emitf("if ")
				}
				g.emitExpr(arm.Guard, false)
				g.buf.WriteString(" {\n")
				g.indent++
				g.emitReturnExpr(arm.Body)
				g.indent--
			} else {
				if i > 0 {
					g.buf.WriteString("{\n")
					g.indent++
					g.emitReturnExpr(arm.Body)
					g.indent--
				} else {
					g.emitReturnExpr(arm.Body)
				}
			}
		}
		// Close the if/else chain block
		// Single unguarded arm: no block to close (just return)
		// Single guarded arm: close `if guard {`
		// Multiple arms: close the last `} else {` block
		if len(grp.arms) > 1 || (len(grp.arms) == 1 && grp.arms[0].Guard != nil) {
			g.emitf("}\n")
		}
		g.indent--
	}

	if !hasDefault {
		g.emitf("default:\n")
		g.indent++
		g.emitf("panic(\"unreachable: unhandled variant\")\n")
		g.indent--
	}

	g.indent--
	g.emitf("}\n")
}

type armGroup struct {
	structName string
	arms       []ast.MatchArm
}

func groupMatchArms(arms []ast.MatchArm, g *Generator) []armGroup {
	var groups []armGroup
	for _, arm := range arms {
		structName := ""
		if cp, ok := arm.Pattern.(*ast.ConstructorPattern); ok {
			structName = g.findVariantStruct(cp.Name)
		}
		if _, ok := arm.Pattern.(*ast.WildcardPattern); ok {
			structName = "default"
		}
		if _, ok := arm.Pattern.(*ast.IdentPattern); ok {
			structName = "default"
		}
		// Merge with last group if same struct name
		if len(groups) > 0 && groups[len(groups)-1].structName == structName {
			groups[len(groups)-1].arms = append(groups[len(groups)-1].arms, arm)
		} else {
			groups = append(groups, armGroup{structName: structName, arms: []ast.MatchArm{arm}})
		}
	}
	return groups
}

func (g *Generator) emitReturnExpr(e ast.Expr) {
	// Check if the enclosing function has a non-unit return type
	hasRet := false
	if g.currentFunc != "" {
		if rt, ok := g.funcRetType[g.currentFunc]; ok && rt != "struct{}" {
			hasRet = true
		}
	}
	if !hasRet {
		// No return type: emit as a statement.
		// Skip unit literals — they become `struct{}{}` which Go forbids
		// as a bare expression statement.
		if lit, ok := e.(*ast.LitExpr); ok && lit.Kind == token.UNIT {
			return
		}
		g.emitExpr(e, true)
		return
	}

	// Emit as a return statement if it's not already a statement
	switch e.(type) {
	case *ast.IfExpr, *ast.MatchExpr, *ast.LetInExpr, *ast.AsMatchExpr, *ast.GuardExpr:
		// These emit their own stmt structure
		g.emitExpr(e, true)
	default:
		g.emitf("return ")
		g.emitExpr(e, false)
		g.buf.WriteString("\n")
	}
}

func (g *Generator) emitPatternBinding(p ast.Pattern, prefix string) {
	switch p := p.(type) {
	case *ast.IdentPattern:
		g.emitf("%s := %s\n", p.Name, prefix)
	case *ast.WildcardPattern:
		// nothing
	case *ast.RecordPattern:
		for _, f := range p.Fields {
			fieldName := prefix + "." + exported(f.Name)
			if f.Pattern != nil {
				g.emitPatternBinding(f.Pattern, fieldName)
			} else {
				g.emitf("%s := %s\n", f.Name, fieldName)
			}
		}
	case *ast.ConstructorPattern:
		// For nested constructor patterns in result arms, bind via the struct field
		structName := g.findVariantStruct(p.Name)
		if structName != "" {
			// This is an ADT variant — bind using the struct field name
			fieldName := g.adtVariantField(p.Name)
			if p.Arg != nil {
				g.emitPatternBinding(p.Arg, prefix+"."+fieldName)
			}
		} else {
			g.emitPatternBinding(p.Arg, prefix)
		}
	}
}

func (g *Generator) findVariantStruct(ctorName string) string {
	// Search through ADTs to find which variant struct contains this constructor
	for _, td := range g.adts {
		ak := td.Kind.(*ast.ADTTypeKind)
		for _, c := range ak.Cases {
			if c.Name == ctorName {
				return g.goName(td.Name) + exported(c.Name)
			}
		}
	}
	return ""
}

// isVariantRecordPayload checks if the named ADT variant constructor
// has an inline record payload (e.g., "Circle of { radius: float }").
// Used to determine whether constructor args should emit record fields
// as individual arguments or pass the record expression as-is.
func (g *Generator) isVariantRecordPayload(ctorName string) bool {
	for _, td := range g.adts {
		ak := td.Kind.(*ast.ADTTypeKind)
		for _, c := range ak.Cases {
			if c.Name == ctorName && c.Arg != nil {
				_, ok := c.Arg.(*ast.TRecord)
				return ok
			}
		}
	}
	return false
}

// adtVariantField returns the struct field name for a constructor's payload.
func (g *Generator) adtVariantField(ctorName string) string {
	for _, td := range g.adts {
		ak := td.Kind.(*ast.ADTTypeKind)
		for _, c := range ak.Cases {
			if c.Name == ctorName && c.Arg != nil {
				return g.variantFieldName(c.Arg, td.Name, c.Name)
			}
		}
	}
	return "Value"
}

// isCustomPreludeStmt checks if an expression is a call to a prelude function
// that uses a Custom lowering which emits a statement rather than a value
// expression (e.g., assert, Chan.send, Chan.close).
// Custom lowerings that return values (e.g., Chan.make, Chan.recv) are NOT
// considered statements by this function.
func isCustomPreludeStmt(e ast.Expr) bool {
	app, ok := e.(*ast.AppExpr)
	if !ok {
		return false
	}
	// Collect the full function chain
	funcExpr := app.Func
	for {
		if inner, ok := funcExpr.(*ast.AppExpr); ok {
			funcExpr = inner.Func
		} else {
			break
		}
	}
	// Check both simple names (e.g., assert) and qualified names (e.g., Chan.send)
	pre := prelude.Default()
	var name string
	if ident, ok := funcExpr.(*ast.IdentExpr); ok {
		name = ident.Name
	}
	if field, ok := funcExpr.(*ast.FieldAccessExpr); ok {
		// Reconstruct the qualified name (e.g., "Chan.send")
		if ctor, ok := field.Left.(*ast.ConstructorExpr); ok && ctor.Arg == nil {
			name = ctor.Name + "." + field.Field
		} else if ident, ok := field.Left.(*ast.IdentExpr); ok {
			name = ident.Name + "." + field.Field
		}
	}
	if name == "" {
		return false
	}
	if b := pre.Lookup(name); b != nil && b.Lowering.Custom != "" {
		// Only match statement-emitting custom lowerings (not value-returning ones)
		switch b.Lowering.Custom {
		case "assert", "assert_equal", "chan_send", "chan_close", "owned_chan_send", "owned_chan_close":
			return true
		}
	}
	return false
}

func (g *Generator) emitLetIn(e *ast.LetInExpr) {
	for _, b := range e.Bindings {
		// Check if the body contains a ? operator
		if qe, ok := b.Body.(*ast.QuestionExpr); ok {
			// Generate: tmp := expr; if !tmp.IsOk() { return tmp }; x := tmp.MustOk()
			g.varCounter++
			tmp := fmt.Sprintf("_tmp%d", g.varCounter)
			g.emitf("%s := ", tmp)
			g.emitExpr(qe.Left, false)
			g.buf.WriteString("\n")
			retType, hasResult := g.funcRetType[g.currentFunc]
			if hasResult && strings.HasPrefix(retType, "Result") {
				g.emitf("if !%s.IsOk() {\n", tmp)
				g.indent++
				g.emitf("return %s\n", tmp)
				g.indent--
				g.emitf("}\n")
			}
			g.emitf("%s := %s.MustOk()\n", b.Name, tmp)
		} else {
			// Check if the body is a prelude call with a custom lowering
			// that emits a statement (e.g., assert, assert_equal).
			// In that case, emit the statement first, then assign struct{}{}.
			if isCustomPreludeStmt(b.Body) {
				g.emitExpr(b.Body, true)
				g.emitf("%s := struct{}{}\n", b.Name)
				g.emitf("_ = %s\n", b.Name)
			} else if g.isChanMakeExpr(b.Body) {
				// let ch = Chan.make () → ch := C0ChanMake()
				g.usedChan = true
				g.emitf("%s := C0ChanMake()\n", b.Name)
			} else {
				g.emitf("%s := ", b.Name)
				g.emitExpr(b.Body, false)
				g.buf.WriteString("\n")
			}
		}
	}
	// Emit the body. Use return only if the enclosing function has a non-unit return type.
	hasRet := false
	if g.currentFunc != "" {
		if rt, ok := g.funcRetType[g.currentFunc]; ok && rt != "struct{}" {
			hasRet = true
		}
	}
	if hasRet {
		g.emitReturnExpr(e.Body)
	} else {
		// No return type: emit as a statement.
		// Skip unit literals — they become `struct{}{}` which Go forbids
		// as a bare expression statement.
		if lit, ok := e.Body.(*ast.LitExpr); ok && lit.Kind == token.UNIT {
			// nothing — void return
		} else {
			g.emitExpr(e.Body, true)
			g.buf.WriteString("\n")
		}
	}
}

func (g *Generator) emitFun(e *ast.FunExpr) {
	g.buf.WriteString("func(")
	for i, p := range e.Params {
		if i > 0 {
			g.buf.WriteString(", ")
		}
		g.buf.WriteString(p.Name)
		g.buf.WriteString(" ")
		if p.Type != nil {
			g.buf.WriteString(g.typeToGo(p.Type))
		} else {
			g.buf.WriteString("interface{}")
		}
	}
	g.buf.WriteString(") interface{} {\n")
	g.indent++
	g.emitf("return ")
	g.emitExpr(e.Body, false)
	g.buf.WriteString("\n")
	g.indent--
	g.emitf("}")
}

func (g *Generator) emitBinary(e *ast.BinaryExpr) {
	// Handle special operators
	switch e.Op {
	case token.CONS:
		// x :: xs → append([]T{x}, xs...)
		// Determine the element type from the right side's context.
		elemType := g.currentListElemType()
		g.buf.WriteString("append([]" + elemType + "{")
		g.emitExpr(e.Left, false)
		g.buf.WriteString("}, ")
		g.emitExpr(e.Right, false)
		g.buf.WriteString("...)")
		return
	case token.STARDOT:
		// *. → *
		g.emitExpr(e.Left, false)
		g.buf.WriteString(" * ")
		g.emitExpr(e.Right, false)
		return
	case token.PLUSDOT:
		// +. → +
		g.emitExpr(e.Left, false)
		g.buf.WriteString(" + ")
		g.emitExpr(e.Right, false)
		return
	case token.MINUSDOT:
		// -. → -
		g.emitExpr(e.Left, false)
		g.buf.WriteString(" - ")
		g.emitExpr(e.Right, false)
		return
	case token.SLASHDOT:
		// /. → /
		g.emitExpr(e.Left, false)
		g.buf.WriteString(" / ")
		g.emitExpr(e.Right, false)
		return
	case token.CARET:
		// ^ (string concat) → +
		g.emitExpr(e.Left, false)
		g.buf.WriteString(" + ")
		g.emitExpr(e.Right, false)
		return
	case token.EQUALS:
		// = (equality) → ==
		g.emitExpr(e.Left, false)
		g.buf.WriteString(" == ")
		g.emitExpr(e.Right, false)
		return
	case token.DIAMOND:
		// <> → !=
		g.emitExpr(e.Left, false)
		g.buf.WriteString(" != ")
		g.emitExpr(e.Right, false)
		return
	}

	g.emitExpr(e.Left, false)
	g.buf.WriteString(" " + e.Op.String() + " ")
	g.emitExpr(e.Right, false)
}

func (g *Generator) emitPipe(e *ast.PipeExpr) {
	// x |> f → f(x)
	g.emitExpr(e.Right, false)
	g.buf.WriteString("(")
	g.emitExpr(e.Left, false)
	g.buf.WriteString(")")
}

func (g *Generator) emitQuestion(e *ast.QuestionExpr) {
	// e ? → result propagation pattern
	g.varCounter++
	tmp := fmt.Sprintf("_tmp%d", g.varCounter)
	g.emitExpr(e.Left, false)
	g.buf.WriteString("\n")
	// The assignment to tmp is done INSIDE the let-in emitter, 
	// so we just generate the value expression here.
	// Actually, the QuestionExpr IS the right-hand side of a let binding.
	// We generate: tmp := expr; if !tmp.IsOk() { return tmp }; x := tmp.MustOk()
	// But the let-binding already generates `x := ...`, so we need to
	// generate the full expanded form here and not also emit `x :=` outside.
	_ = tmp
}

// emitIsExpr generates Go code for `expr is pattern` (match macro).
// Uses an immediately-invoked function expression for the type switch.
func (g *Generator) emitIsExpr(e *ast.IsExpr) {
	fakeMatch := &ast.MatchExpr{
		Scrutinee: e.Left,
		Arms: []ast.MatchArm{
			{Pattern: e.Pattern, Body: &ast.LitExpr{Value: true, Kind: token.TRUE}},
			{Pattern: &ast.WildcardPattern{}, Body: &ast.LitExpr{Value: false, Kind: token.FALSE}},
		},
	}
	g.buf.WriteString("func() bool {\n")
	g.indent++
	g.emitMatch(fakeMatch)
	g.indent--
	g.emitf("}()")
}

// emitAsMatchExpr generates Go for `expr as pattern -> then else elseExpr`.
func (g *Generator) emitAsMatchExpr(e *ast.AsMatchExpr) {
	fakeMatch := &ast.MatchExpr{
		Scrutinee: e.Left,
		Arms: []ast.MatchArm{
			{Pattern: e.Pattern, Body: e.Body},
			{Pattern: &ast.WildcardPattern{}, Body: e.ElseBody},
		},
	}
	g.emitMatch(fakeMatch)
}

func (g *Generator) emitGoExpr(e *ast.GoExpr) {
	g.buf.WriteString("go ")
	// Unwrap ParenExpr to check for FunExpr inside: `go (fun () -> ...)`
	inner := e.Expr
	if pe, ok := inner.(*ast.ParenExpr); ok {
		inner = pe.Inner
	}
	if _, ok := inner.(*ast.FunExpr); ok {
		// emitFun generates `func(...) ...` — just add `go` prefix
		g.emitExpr(inner, false)
		g.buf.WriteString("()")
	} else {
		g.buf.WriteString("func() { ")
		g.emitExpr(inner, false)
		g.buf.WriteString(" }()")
	}
}

func (g *Generator) emitSelectExpr(e *ast.SelectExpr) {
	g.emitf("select {\n")
	g.indent++
	for _, c := range e.Cases {
		// Assume receive expression
		g.emitf("case %s := <-", c.Bind)
		g.emitExpr(c.Recv, false)
		g.buf.WriteString(":\n")
		g.indent++
		g.emitReturnExpr(c.Body)
		g.indent--
	}
	if e.Default != nil {
		g.emitf("default:\n")
		g.indent++
		g.emitReturnExpr(e.Default)
		g.indent--
	}
	g.indent--
	g.emitf("}\n")
}

func (g *Generator) emitUsing(e *ast.UsingExpr, isStmt bool) {
	// using x = e in body → x := e; defer Close(x); body
	// The cleanup function Close must be in scope (v1 convention).
	varName := "v"
	if ip, ok := e.Pattern.(*ast.IdentPattern); ok {
		varName = ip.Name
	}

	if !isStmt {
		g.buf.WriteString("func() interface{} {\n")
		g.indent++
	}

	g.emitf("%s := ", varName)
	g.emitExpr(e.Expr, false)
	g.buf.WriteString("\n")
	g.emitf("defer Close(%s)\n", varName)
	g.emitReturnExpr(e.Body)

	if !isStmt {
		g.indent--
		g.emitf("}()\n")
	}
}

// emitRegion emits a `region { ops }` block inline (no closure wrapper).
// Each let! binding becomes: varName := expr; defer Close(varName)
// Regular let bindings become: varName := expr (no defer).
// The result is the value of the final return/return!/body expression.
func (g *Generator) emitRegion(e *ast.RegionExpr) {
	for _, op := range e.Ops {
		switch o := op.(type) {
		case *ast.LetBangOp:
			varName := "v"
			if ip, ok := o.Pattern.(*ast.IdentPattern); ok {
				varName = ip.Name
			}
			g.emitf("%s := ", varName)
			g.emitExpr(o.Expr, false)
			g.buf.WriteString("\n")
			g.emitf("defer Close(%s)\n", varName)
		case *ast.LetOp:
			varName := "v"
			if ip, ok := o.Pattern.(*ast.IdentPattern); ok {
				varName = ip.Name
			}
			g.emitf("%s := ", varName)
			g.emitExpr(o.Expr, false)
			g.buf.WriteString("\n")
		case *ast.DoBangOp:
			g.emitExpr(o.Expr, true)
			g.buf.WriteString("\n")
		case *ast.ReturnOp:
			g.emitReturnExpr(o.Expr)
		case *ast.ReturnBangOp:
			g.emitReturnExpr(o.Expr)
		case *ast.BodyOp:
			g.emitReturnExpr(o.Expr)
		}
	}
}

func (g *Generator) emitRecord(e *ast.RecordExpr) {
	// Determine the record type from fields
	recName := g.inferRecordName(e)
	g.buf.WriteString(recName + "{")
	for i, f := range e.Fields {
		if i > 0 {
			g.buf.WriteString(", ")
		}
		g.buf.WriteString(exported(f.Name) + ": ")
		if f.Value != nil {
			g.emitExpr(f.Value, false)
		} else {
			// Field punning
			g.buf.WriteString(f.Name)
		}
	}
	g.buf.WriteString("}")
}

func (g *Generator) inferRecordName(e *ast.RecordExpr) string {
	// Match against known record types
	for name, td := range g.records {
		rk := td.Kind.(*ast.RecordTypeKind)
		if len(rk.Fields) == len(e.Fields) {
			match := true
			for i, f := range rk.Fields {
				if i >= len(e.Fields) || f.Name != e.Fields[i].Name {
					match = false
					break
				}
			}
			if match {
				return g.goName(name)
			}
		}
	}
	return "struct{...}"
}

func (g *Generator) emitRecordUpdate(e *ast.RecordUpdateExpr) {
	// { r with x = e } → reconstruct struct
	// Determine the record type from the base expression
	recName := g.inferRecordUpdateName(e)
	g.buf.WriteString(recName + "{\n")
	g.indent++
	// Copy all fields from base: for each known field, emit Field: base.Field
	// Then override with specified fields
	// Simplified: emit all known fields with base values, then overrides
	td := g.findRecordType(recName)
	if td != nil {
		rk := td.Kind.(*ast.RecordTypeKind)
		for _, f := range rk.Fields {
			// Check if this field is being overridden
			overridden := false
			for _, of := range e.Fields {
				if of.Name == f.Name {
					overridden = true
					break
				}
			}
			if overridden {
				continue
			}
			g.emitf("%s: ", exported(f.Name))
			g.emitExpr(e.Base, false)
			g.buf.WriteString("." + exported(f.Name) + ",\n")
		}
	}
	// Emit overridden fields
	for _, f := range e.Fields {
		g.emitf("%s: ", exported(f.Name))
		g.emitExpr(f.Value, false)
		g.buf.WriteString(",\n")
	}
	// Remove trailing comma by seeking back
	g.indent--
	g.emitf("}")
}

func (g *Generator) inferRecordUpdateName(e *ast.RecordUpdateExpr) string {
	// Try to determine from the base expression's type
	// The base is typically an identifier
	if ident, ok := e.Base.(*ast.IdentExpr); ok {
		// Look up the parameter or variable's type
		// This is an approximation — check known records
		for name := range g.records {
			if g.goName(name) == g.goName(ident.Name) || name == ident.Name {
				return g.goName(name)
			}
		}
	}
	return "struct{...}"
}

func (g *Generator) findRecordType(goName string) *ast.TypeDecl {
	for name, td := range g.records {
		if g.goName(name) == goName {
			return td
		}
	}
	return nil
}

// fieldAccessName returns the dotted name for a simple field access expression
// like Console.print_line, or empty string if it's complex.
func (g *Generator) fieldAccessName(e *ast.FieldAccessExpr) string {
	if ctor, ok := e.Left.(*ast.ConstructorExpr); ok && ctor.Arg == nil {
		return ctor.Name + "." + e.Field
	}
	if ident, ok := e.Left.(*ast.IdentExpr); ok {
		return ident.Name + "." + e.Field
	}
	return ""
}

func (g *Generator) emitFieldAccess(e *ast.FieldAccessExpr) {
	// If left is an ident and the current function has a row parameter
	// with that name, access the expanded Go param directly.
	if ident, ok := e.Left.(*ast.IdentExpr); ok {
		if rowName, hasRow := g.rowParamName[g.currentFunc]; hasRow && rowName == ident.Name {
			g.buf.WriteString(ident.Name + "_" + e.Field)
			return
		}
	}
	// If left is a constructor (like Console), emit specially
	if ctor, ok := e.Left.(*ast.ConstructorExpr); ok {
		if ctor.Name == "Console" && ctor.Arg == nil {
			if e.Field == "print_line" {
				g.buf.WriteString("fmt.Println")
				g.needFmt = true
				return
			}
		}
	}
	g.emitExpr(e.Left, false)
	g.buf.WriteString(".")
	g.buf.WriteString(exported(e.Field))
}

func (g *Generator) emitTuple(e *ast.TupleExpr) {
	parts := make([]string, len(e.Elems))
	for i, el := range e.Elems {
		var buf strings.Builder
		old := g.buf
		g.buf = buf
		g.emitExpr(el, false)
		parts[i] = g.buf.String()
		g.buf = old
	}
	g.buf.WriteString("Tuple" + strings.Join(parts, "") + "{")
	for i, p := range parts {
		if i > 0 {
			g.buf.WriteString(", ")
		}
		g.buf.WriteString("F" + fmt.Sprintf("%d", i) + ": " + p)
	}
	g.buf.WriteString("}")
}

func (g *Generator) emitList(e *ast.ListExpr) {
	if len(e.Elems) == 0 {
		g.buf.WriteString("nil")
		return
	}
	// Determine element type from enclosing function's return type if possible
	elemType := g.currentListElemType()
	g.buf.WriteString("[]" + elemType + "{")
	for i, el := range e.Elems {
		if i > 0 {
			g.buf.WriteString(", ")
		}
		g.emitExpr(el, false)
	}
	g.buf.WriteString("}")
}

func (g *Generator) currentListElemType() string {
	if ret, ok := g.funcRetType[g.currentFunc]; ok {
		if strings.HasPrefix(ret, "[]") {
			return ret[2:]
		}
	}
	return "interface{}"
}

// ---------------------------------------------------------------------------
// Active pattern match lowering
// ---------------------------------------------------------------------------

func (g *Generator) hasActivePattern(e *ast.MatchExpr) bool {
	for _, arm := range e.Arms {
		if cp, ok := arm.Pattern.(*ast.ConstructorPattern); ok {
			if active.GlobalRegistry.IsActivePattern(cp.Name) {
				return true
			}
		}
	}
	return false
}

func (g *Generator) emitActiveMatch(e *ast.MatchExpr) {
	// Lower to flat if statements: for each active pattern arm,
	//   if _opt := __active_Positive(v); _opt.IsSome() { ... }
	// Guarded arms: if guard fails, fall through to next arm.
	// Wildcard arm: emit as final else or bare return.
	g.varCounter++
	scrutVar := fmt.Sprintf("_a%d", g.varCounter)
	g.emitf("%s := ", scrutVar)
	g.emitExpr(e.Scrutinee, false)
	g.buf.WriteString("\n")

	for i, arm := range e.Arms {
		switch p := arm.Pattern.(type) {
		case *ast.ConstructorPattern:
			if ap := active.GlobalRegistry.Lookup(p.Name); ap != nil {
				goFunc := ap.GoFuncName
				// Store the option result in a temp to avoid double calls
				g.varCounter++
				optVar := fmt.Sprintf("_opt%d", g.varCounter)
				g.emitf("if %s := %s(%s); %s.IsSome() {\n",
					optVar, goFunc, scrutVar, optVar)
				g.indent++
				if p.Arg != nil {
					varName := g.patternVarName(p.Arg)
					// Use = for blank identifier (Go 1.22 doesn't allow _ := expr alone)
					if varName == "_" {
						g.emitf("_ = %s.MustSome()\n", optVar)
					} else {
						g.emitf("%s := %s.MustSome()\n", varName, optVar)
						// Suppress "declared and not used"
						g.emitf("_ = %s\n", varName)
					}
				}
				if arm.Guard != nil {
					g.emitf("if ")
					g.emitExpr(arm.Guard, false)
					g.buf.WriteString(" {\n")
					g.indent++
					g.emitReturnExpr(arm.Body)
					g.indent--
					g.emitf("}\n")
				} else {
					g.emitReturnExpr(arm.Body)
				}
				g.indent--
				g.emitf("}\n")
			}
		case *ast.WildcardPattern:
			// Wildcard: emit as the final return
			g.emitReturnExpr(arm.Body)
			return // done — nothing after wildcard
		case *ast.IdentPattern:
			// Variable pattern: bind and return
			g.emitf("%s := %s\n", p.Name, scrutVar)
			g.emitReturnExpr(arm.Body)
			return
		}
		_ = i
	}

	// If we get here, there was no wildcard. Emit a panic as fallback.
	g.emitf("panic(\"unreachable: unhandled active pattern\")\n")
}

// ---------------------------------------------------------------------------
// Refinement contract checks
// ---------------------------------------------------------------------------

// emitPreconditionCheck emits a runtime check for a parameter refinement:
//   if !(<pred>) { panic("<func>: precondition violated: <pred text>") }
func (g *Generator) emitPreconditionCheck(funcName, paramName string, rt *ast.RefinementType) {
	var predBuf strings.Builder
	origBuf := g.buf
	g.buf = predBuf
	g.emitExpr(rt.Pred, false)
	predGo := g.buf.String()
	g.buf = origBuf

	g.emitf("if !(%s) { panic(\"%s: precondition violated: %s\") }\n",
		predGo, funcName, g.predicateSource(rt))
}

// emitPostconditionCheck emits a defer check for a return type refinement:
//   defer func() { if !(<pred>) { panic("<func>: postcondition violated") } }()
func (g *Generator) emitPostconditionCheck(funcName string, rt *ast.RefinementType) {
	var predBuf strings.Builder
	origBuf := g.buf
	g.buf = predBuf
	g.emitExpr(rt.Pred, false)
	predGo := g.buf.String()
	g.buf = origBuf

	g.emitf("defer func() {\n")
	g.indent++
	g.emitf("if !(%s) { panic(\"%s: postcondition violated: %s\") }\n",
		predGo, funcName, g.predicateSource(rt))
	g.indent--
	g.emitf("}()\n")
}

// predicateSource returns a human-readable form of the refinement predicate.
func (g *Generator) predicateSource(rt *ast.RefinementType) string {
	return g.exprToSource(rt.Pred)
}

// exprToSource recursively converts an expression to a readable source form.
func (g *Generator) exprToSource(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.BinaryExpr:
		left := g.exprToSource(e.Left)
		right := g.exprToSource(e.Right)
		op := e.Op.String()
		return left + " " + op + " " + right
	case *ast.IdentExpr:
		return e.Name
	case *ast.LitExpr:
		return fmt.Sprintf("%v", e.Value)
	default:
		return ast.ExprString(e)
	}
}

// resolveChanElemType attempts to determine the Go type of a channel's
// element by inspecting the TypeMap. It relies on the type checker having
// fully resolved all type variables (see CheckWithTypes in typecheck.go).
func (g *Generator) resolveChanElemType(args []ast.Expr) string {
	if g.typeMap == nil || len(args) == 0 {
		return ""
	}
	// Look up the channel argument's type in the TypeMap.
	// After type checking, this should be a concrete *types.TChan.
	if chanType, ok := g.typeMap[args[0]].(*types.TChan); ok {
		elemGo := g.internalTypeToGo(chanType.Elem)
		if elemGo != "interface{}" {
			return elemGo
		}
	}
	// Also handle owned_chan (TAdt) — same *C0Chan Go type.
	if adtType, ok := g.typeMap[args[0]].(*types.TAdt); ok && adtType.Name == "owned_chan" {
		if len(adtType.Params) > 0 {
			elemGo := g.internalTypeToGo(adtType.Params[0])
			if elemGo != "interface{}" {
				return elemGo
			}
		}
	}
	// Fallback: if the argument is an identifier, look it up in the VarTypeMap
	if ident, ok := args[0].(*ast.IdentExpr); ok {
		if t, ok := g.varTypeMap[ident.Name]; ok {
			if tc, ok := t.(*types.TChan); ok {
				elemGo := g.internalTypeToGo(tc.Elem)
				if elemGo != "interface{}" {
					return elemGo
				}
			}
			// Also handle owned_chan in VarTypeMap
			if adt, ok := t.(*types.TAdt); ok && adt.Name == "owned_chan" {
				if len(adt.Params) > 0 {
					elemGo := g.internalTypeToGo(adt.Params[0])
					if elemGo != "interface{}" {
						return elemGo
					}
				}
			}
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Channel wrapper helpers
// ---------------------------------------------------------------------------

// emitChanHelpers emits the C0Chan wrapper struct and its helper functions
// if any channel operations are used in the module.
func (g *Generator) emitChanHelpers() {
	if !g.usedChan {
		return
	}
	g.emitf("// C0 file-operations‑safe channel wrapper with close‑time tracking\n")
	g.emitf("type C0Chan struct {\n")
	g.indent++
	g.emitf("ch     chan interface{}\n")
	g.emitf("closed bool\n")
	g.indent--
	g.emitf("}\n\n")

	g.emitf("func C0ChanMake() *C0Chan {\n")
	g.indent++
	g.emitf("return &C0Chan{ch: make(chan interface{})}\n")
	g.indent--
	g.emitf("}\n\n")

	g.emitf("func C0ChanSend(c *C0Chan, v interface{}) {\n")
	g.indent++
	g.emitf("if c.closed { panic(\"Chan.send: channel is closed\") }\n")
	g.emitf("c.ch <- v\n")
	g.indent--
	g.emitf("}\n\n")

	g.emitf("func C0ChanRecv(c *C0Chan) interface{} {\n")
	g.indent++
	g.emitf("return <-c.ch\n")
	g.indent--
	g.emitf("}\n\n")

	g.emitf("func C0ChanClose(c *C0Chan) {\n")
	g.indent++
	g.emitf("if c.closed { panic(\"Chan.close: channel already closed\") }\n")
	g.emitf("c.closed = true\n")
	g.emitf("close(c.ch)\n")
	g.indent--
	g.emitf("}\n\n")
}

// ---------------------------------------------------------------------------
// Extern declarations
// ---------------------------------------------------------------------------

func (g *Generator) emitExternDecl(d *ast.ExternDecl) {
	g.emitf("// extern %q %q\n", d.Lang, d.Path)
	for _, ev := range d.Vals {
		g.emitf("// val %s : %s\n", ev.Name, g.typeToGo(ev.Type))
	}
	g.buf.WriteString("\n")
}
