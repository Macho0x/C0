// Package gosig provides an optional fallback for querying real Go function
// signatures using golang.org/x/tools/go/packages and go/types. This helps
// the Goop typechecker resolve lambda parameter types when Goop's own
// Hindley-Milner inference cannot determine them from local context alone.
package gosig

import (
	"context"
	"fmt"
	"sync"
	"time"

	gotypes "go/types"

	"golang.org/x/tools/go/packages"
)

// Param represents a Go function parameter with its name and Go type string.
type Param struct {
	Name string // parameter name (may be empty for unnamed params)
	Type string // Go type as a string, e.g. "int", "string", "func(int, int) bool"
}

// FuncSig holds the full Go function signature: parameter types and result types.
type FuncSig struct {
	Params  []Param
	Results []Param // result types (usually 0 or 1 for Goop FFI)
}

// cache avoids reloading the same package multiple times.
var cache sync.Map // key "importPath.funcName" → cachedResult

type cachedResult struct {
	sig *FuncSig
	err error
}

// LookupFunc loads a Go package via packages.Load and looks up a function's
// parameter and result types. Results are cached in memory keyed by
// (importPath, funcName). A 5-second timeout prevents hung loads from
// blocking the compiler.
func LookupFunc(importPath, funcName string) (*FuncSig, error) {
	key := importPath + "." + funcName
	if cached, ok := cache.Load(key); ok {
		cr := cached.(cachedResult)
		return cr.sig, cr.err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedModule,
		Context: ctx,
	}

	pkgs, err := packages.Load(cfg, importPath)
	if err != nil {
		cr := cachedResult{err: fmt.Errorf("loading package %q: %w", importPath, err)}
		cache.Store(key, cr)
		return nil, cr.err
	}

	if len(pkgs) == 0 {
		cr := cachedResult{err: fmt.Errorf("no packages found for import path %q", importPath)}
		cache.Store(key, cr)
		return nil, cr.err
	}

	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		cr := cachedResult{err: fmt.Errorf("package %q has errors: %v", importPath, pkg.Errors[0])}
		cache.Store(key, cr)
		return nil, cr.err
	}

	if pkg.Types == nil || pkg.Types.Scope() == nil {
		cr := cachedResult{err: fmt.Errorf("package %q has no type information (try adding it to go.mod?)", importPath)}
		cache.Store(key, cr)
		return nil, cr.err
	}

	obj := pkg.Types.Scope().Lookup(funcName)
	if obj == nil {
		cr := cachedResult{err: fmt.Errorf("function %q not found in package %q", funcName, importPath)}
		cache.Store(key, cr)
		return nil, cr.err
	}

	fn, ok := obj.(*gotypes.Func)
	if !ok {
		cr := cachedResult{err: fmt.Errorf("%q is not a function (got %T)", funcName, obj)}
		cache.Store(key, cr)
		return nil, cr.err
	}

	sig, ok := fn.Type().(*gotypes.Signature)
	if !ok {
		cr := cachedResult{err: fmt.Errorf("%q has non-signature type %T", funcName, fn.Type())}
		cache.Store(key, cr)
		return nil, cr.err
	}

	qual := relativeQualifier(importPath)

	fsig := &FuncSig{}

	params := sig.Params()
	fsig.Params = make([]Param, params.Len())
	for i := 0; i < params.Len(); i++ {
		p := params.At(i)
		fsig.Params[i] = Param{
			Name: p.Name(),
			Type: gotypes.TypeString(p.Type(), qual),
		}
	}

	results := sig.Results()
	fsig.Results = make([]Param, results.Len())
	for i := 0; i < results.Len(); i++ {
		r := results.At(i)
		fsig.Results[i] = Param{
			Name: r.Name(),
			Type: gotypes.TypeString(r.Type(), qual),
		}
	}

	cr := cachedResult{sig: fsig}
	cache.Store(key, cr)
	return fsig, nil
}

// relativeQualifier returns a types.Qualifier that strips the current package
// path from type names, making them more suitable for Goop type mapping.
func relativeQualifier(importPath string) gotypes.Qualifier {
	return func(pkg *gotypes.Package) string {
		if pkg.Path() == importPath {
			return ""
		}
		return pkg.Name()
	}
}
