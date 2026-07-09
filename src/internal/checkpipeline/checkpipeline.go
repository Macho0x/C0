// Package checkpipeline runs post-typecheck safety analyses in a fixed order.
package checkpipeline

import (
	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/config"
	"goop.dev/compiler/internal/exhaustive"
	"goop.dev/compiler/internal/linear"
	"goop.dev/compiler/internal/nilchan"
	"goop.dev/compiler/internal/refine"
	"goop.dev/compiler/internal/typeinfo"
)

// Result holds outcomes from all safety passes.
type Result struct {
	LinearErrors   []error
	NilchanErrors  []error
	RefineProven   refine.ProvenSites
	RefineWarnings []error
	RefineErrors   []error
	ExhaustErrors  []error
	ExhaustWarns   []error
}

// Run executes linear, nil-channel, refinement, and exhaustiveness checks.
func Run(mod *ast.Module, tm typeinfo.TypeMap, linearTypes map[string]bool, cfg *config.Config) Result {
	var r Result
	r.LinearErrors = linear.Check(mod, linearTypes)
	r.NilchanErrors = nilchan.Check(mod)
	r.RefineProven, r.RefineWarnings, r.RefineErrors = refine.CheckRefinements(mod, tm)
	exErrs, exWarns := exhaustive.CheckWithConfig(mod, cfg)
	r.ExhaustErrors = exErrs
	r.ExhaustWarns = exWarns
	return r
}

// BuildLinearTypes extracts linear type names from a module.
func BuildLinearTypes(mod *ast.Module) map[string]bool {
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
	lt["owned_chan"] = true
	return lt
}

// RegisterADTsFromModule populates the exhaustive ADT registry from type decls.
func RegisterADTsFromModule(mod *ast.Module) {
	exhaustive.ADTRegistry = map[string][]string{
		"result": {"Ok", "Error"},
		"option": {"None", "Some"},
	}
	for _, d := range mod.Decls {
		td, ok := d.(*ast.TypeDecl)
		if !ok {
			continue
		}
		adt, ok := td.Kind.(*ast.ADTTypeKind)
		if !ok {
			continue
		}
		var ctors []string
		for _, c := range adt.Cases {
			ctors = append(ctors, c.Name)
		}
		exhaustive.RegisterADT(td.Name, ctors)
	}
}
