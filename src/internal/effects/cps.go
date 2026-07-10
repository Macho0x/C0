// Package effects lowers OCaml-style effect handlers to a minimal CPS form.
package effects

import "goop.dev/compiler/internal/ast"

// TransformCPS rewrites modules that use perform / effect handlers into a
// continuation-passing style suitable for Go codegen.
//
// Minimal strategy:
//   - Mark functions whose bodies contain perform or effect-handler matches
//   - Rewrite `perform e` to `__goop_perform(e)`
//   - Rewrite effect-handler match arms to `__goop_handle` applications
// Pure functions are left unchanged.
func TransformCPS(mod *ast.Module) *ast.Module {
	if mod == nil || !moduleUsesEffects(mod) {
		return mod
	}
	for _, d := range mod.Decls {
		transformDecl(d)
	}
	return mod
}

func moduleUsesEffects(mod *ast.Module) bool {
	found := false
	var walkExpr func(ast.Expr)
	walkExpr = func(e ast.Expr) {
		if e == nil || found {
			return
		}
		switch e := e.(type) {
		case *ast.PerformExpr:
			found = true
		case *ast.MatchExpr:
			for _, a := range e.Arms {
				if a.EffectHandler {
					found = true
					return
				}
				walkExpr(a.Body)
				walkExpr(a.Guard)
			}
			walkExpr(e.Scrutinee)
		case *ast.AppExpr:
			walkExpr(e.Func)
			walkExpr(e.Arg)
		case *ast.IfExpr:
			walkExpr(e.Cond)
			walkExpr(e.ThenBranch)
			walkExpr(e.ElseBranch)
		case *ast.LetInExpr:
			for _, b := range e.Bindings {
				walkExpr(b.Body)
			}
			walkExpr(e.Body)
		case *ast.FunExpr:
			walkExpr(e.Body)
		case *ast.BeginExpr:
			for _, s := range e.Stmts {
				walkExpr(s)
			}
		case *ast.BinaryExpr:
			walkExpr(e.Left)
			walkExpr(e.Right)
		case *ast.GoExpr:
			walkExpr(e.Expr)
		}
	}
	for _, d := range mod.Decls {
		switch d := d.(type) {
		case *ast.LetDecl:
			for _, b := range d.Bindings {
				walkExpr(b.Body)
			}
		case *ast.EffectDecl:
			found = true
		case *ast.NestedModuleDecl:
			for _, nd := range d.Decls {
				if ld, ok := nd.(*ast.LetDecl); ok {
					for _, b := range ld.Bindings {
						walkExpr(b.Body)
					}
				}
			}
		}
		if found {
			return true
		}
	}
	return found
}

func transformDecl(d ast.TopDecl) {
	switch d := d.(type) {
	case *ast.LetDecl:
		for i := range d.Bindings {
			d.Bindings[i].Body = transformExpr(d.Bindings[i].Body)
		}
	case *ast.NestedModuleDecl:
		for _, nd := range d.Decls {
			transformDecl(nd)
		}
	case *ast.ClassDecl:
		for i := range d.Methods {
			d.Methods[i].Body = transformExpr(d.Methods[i].Body)
		}
	}
}

func transformExpr(e ast.Expr) ast.Expr {
	if e == nil {
		return nil
	}
	switch e := e.(type) {
	case *ast.PerformExpr:
		e.Op = transformExpr(e.Op)
		// perform e  →  __goop_perform e
		return &ast.AppExpr{
			Func: &ast.IdentExpr{Name: "__goop_perform", Loc: e.Loc},
			Arg:  e.Op,
			Loc:  e.Loc,
		}
	case *ast.MatchExpr:
		e.Scrutinee = transformExpr(e.Scrutinee)
		hasEffect := false
		for i := range e.Arms {
			e.Arms[i].Body = transformExpr(e.Arms[i].Body)
			if e.Arms[i].Guard != nil {
				e.Arms[i].Guard = transformExpr(e.Arms[i].Guard)
			}
			if e.Arms[i].EffectHandler {
				hasEffect = true
			}
		}
		if hasEffect {
			// Wrap as __goop_handle(scrutinee) — handlers remain in MatchExpr
			// for codegen to emit; mark via Ident wrapper.
			return &ast.AppExpr{
				Func: &ast.IdentExpr{Name: "__goop_handle", Loc: e.Loc},
				Arg:  e,
				Loc:  e.Loc,
			}
		}
		return e
	case *ast.AppExpr:
		e.Func = transformExpr(e.Func)
		e.Arg = transformExpr(e.Arg)
		return e
	case *ast.IfExpr:
		e.Cond = transformExpr(e.Cond)
		e.ThenBranch = transformExpr(e.ThenBranch)
		e.ElseBranch = transformExpr(e.ElseBranch)
		return e
	case *ast.LetInExpr:
		for i := range e.Bindings {
			e.Bindings[i].Body = transformExpr(e.Bindings[i].Body)
		}
		e.Body = transformExpr(e.Body)
		return e
	case *ast.FunExpr:
		e.Body = transformExpr(e.Body)
		return e
	case *ast.BeginExpr:
		for i := range e.Stmts {
			e.Stmts[i] = transformExpr(e.Stmts[i])
		}
		return e
	case *ast.BinaryExpr:
		e.Left = transformExpr(e.Left)
		e.Right = transformExpr(e.Right)
		return e
	case *ast.GoExpr:
		e.Expr = transformExpr(e.Expr)
		return e
	case *ast.LazyExpr:
		e.Value = transformExpr(e.Value)
		return e
	case *ast.LabelledArgExpr:
		if e.Value != nil {
			e.Value = transformExpr(e.Value)
		}
		return e
	case *ast.LetModuleExpr:
		for _, d := range e.Decls {
			transformDecl(d)
		}
		e.Body = transformExpr(e.Body)
		return e
	default:
		return e
	}
}
