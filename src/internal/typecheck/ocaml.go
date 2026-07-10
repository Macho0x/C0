package typecheck

import (
	"os"
	"path/filepath"
	"strings"

	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/parser"
	"goop.dev/compiler/internal/token"
	"goop.dev/compiler/internal/types"
)

// moduleExports tracks bindings introduced by nested modules for open/include.
type moduleExports struct {
	vals map[string]*types.Scheme
}

func (c *Checker) ensureModules() {
	if c.modules == nil {
		c.modules = make(map[string]*moduleExports)
	}
}

// checkTopDeclExtra handles Phase 3–5 declarations.
func (c *Checker) checkTopDeclExtra(d ast.TopDecl) {
	switch d := d.(type) {
	case *ast.NestedModuleDecl:
		c.checkNestedModule(d)
	case *ast.ModuleTypeDecl:
		// Signature stored for documentation / future sealing; no runtime binding.
		_ = d
	case *ast.OpenModuleDecl:
		c.checkOpen(d.Path)
	case *ast.IncludeDecl:
		c.checkOpen(d.Path) // minimal: same as open
	case *ast.ClassDecl:
		c.checkClassDecl(d)
	case *ast.EffectDecl:
		// Register effect name as a constructor-like value for perform.
		from := c.convertASTType(d.From)
		to := c.convertASTType(d.To)
		c.env.Bind(d.Name, types.Mono(&types.TFun{From: from, To: to}))
	}
}

func (c *Checker) checkNestedModule(d *ast.NestedModuleDecl) {
	c.ensureModules()
	saved := c.env
	c.env = NewEnv(c.env)
	ex := &moduleExports{vals: make(map[string]*types.Scheme)}

	if d.IsApp {
		// Functor application / alias: copy exports from AppArg or AppFunc.
		src := d.AppArg
		if src == "" {
			src = d.AppFunc
		}
		if m, ok := c.modules[src]; ok {
			for k, v := range m.vals {
				ex.vals[k] = v
				c.env.Bind(k, v)
			}
		}
	} else {
		// Register nested types first
		for _, decl := range d.Decls {
			if td, ok := decl.(*ast.TypeDecl); ok {
				scheme := c.convertTypeDecl(td)
				c.env.Bind(td.Name, scheme)
			}
		}
		for _, decl := range d.Decls {
			switch decl := decl.(type) {
			case *ast.LetDecl:
				c.checkLetDecl(decl)
				for _, b := range decl.Bindings {
					if s := c.env.Lookup(b.Name); s != nil {
						ex.vals[b.Name] = s
					}
				}
			case *ast.TypeDecl:
				// already registered
			case *ast.ExceptionDecl:
				c.checkExceptionDecl(decl)
			case *ast.NestedModuleDecl:
				c.checkNestedModule(decl)
			case *ast.OpenModuleDecl:
				c.checkOpen(decl.Path)
			case *ast.IncludeDecl:
				c.checkOpen(decl.Path)
			case *ast.ClassDecl:
				c.checkClassDecl(decl)
			case *ast.EffectDecl:
				c.checkTopDeclExtra(decl)
			}
		}
	}

	c.env = saved
	c.modules[d.Name] = ex
	// Bind module name as a unit placeholder so references don't fail hard.
	c.env.Bind(d.Name, types.Mono(types.Unit))
}

func (c *Checker) checkOpen(path string) {
	c.ensureModules()
	// Take last segment as module name
	name := path
	if i := strings.LastIndex(path, "."); i >= 0 {
		name = path[i+1:]
	}
	if m, ok := c.modules[name]; ok {
		for k, v := range m.vals {
			c.env.Bind(k, v)
		}
		return
	}
	// Also try full path key
	if m, ok := c.modules[path]; ok {
		for k, v := range m.vals {
			c.env.Bind(k, v)
		}
	}
}

func (c *Checker) checkClassDecl(d *ast.ClassDecl) {
	fields := make([]types.Field, 0, len(d.Fields)+len(d.Methods))
	for _, f := range d.Fields {
		var ft types.Type = types.Unit
		if f.Value != nil {
			ft = c.infer(f.Value)
		}
		fields = append(fields, types.Field{Name: f.Name, Type: ft})
	}
	saved := c.env
	c.env = NewEnv(c.env)
	if d.Self != "" {
		c.env.Bind(d.Self, types.Mono(&types.TRecord{Fields: fields}))
	}
	for _, m := range d.Methods {
		mt := c.infer(&ast.FunExpr{Params: m.Params, Body: m.Body})
		fields = append(fields, types.Field{Name: m.Name, Type: mt})
	}
	c.env = saved
	rec := &types.TRecord{Fields: fields}
	c.env.Bind(d.Name, types.Mono(&types.TCon{Name: "class", Args: []types.Type{rec}}))
	c.env.Bind("new_"+d.Name, types.Mono(&types.TFun{From: types.Unit, To: rec}))
}

func (c *Checker) convertGADT(td *ast.TypeDecl, k *ast.GADTTypeKind) *types.Scheme {
	variants := make([]types.Variant, len(k.Cases))
	adt := &types.TAdt{Name: td.Name, Variants: variants, Linear: td.Quantity == 1}
	for i, cs := range k.Cases {
		v := types.Variant{Name: cs.Name}
		if cs.Arg != nil {
			v.Arg = c.convertASTType(cs.Arg)
		}
		variants[i] = v
		var ctorType types.Type = adt
		if cs.Arg != nil {
			ctorType = &types.TFun{From: c.convertASTType(cs.Arg), To: adt}
		}
		// Approximate: ignore Result refinement; bind as ordinary ADT ctor.
		_ = cs.Result
		c.env.Bind(cs.Name, types.Mono(ctorType))
	}
	adt.Variants = variants
	return types.Mono(adt)
}

func (c *Checker) inferObject(e *ast.ObjectExpr) types.Type {
	fields := make([]types.Field, 0, len(e.Fields)+len(e.Methods))
	for _, f := range e.Fields {
		var ft types.Type = types.Unit
		if f.Value != nil {
			ft = c.infer(f.Value)
		}
		fields = append(fields, types.Field{Name: f.Name, Type: ft})
	}
	saved := c.env
	c.env = NewEnv(c.env)
	if e.Self != "" {
		c.env.Bind(e.Self, types.Mono(&types.TRecord{Fields: fields}))
	}
	for _, m := range e.Methods {
		mt := c.infer(&ast.FunExpr{Params: m.Params, Body: m.Body})
		fields = append(fields, types.Field{Name: m.Name, Type: mt})
	}
	c.env = saved
	return &types.TRecord{Fields: fields}
}

func (c *Checker) inferNew(e *ast.NewExpr) types.Type {
	s := c.env.Lookup(e.Class)
	if s == nil {
		c.errorfAt(e.Loc, "undefined class %s", e.Class)
		return c.fresh("new")
	}
	t := s.Instantiate()
	if tc, ok := t.(*types.TCon); ok && tc.Name == "class" && len(tc.Args) > 0 {
		return tc.Args[0]
	}
	return t
}

func (c *Checker) inferLetModule(e *ast.LetModuleExpr) types.Type {
	tmp := &ast.NestedModuleDecl{Name: e.Name, Decls: e.Decls}
	c.checkNestedModule(tmp)
	c.checkOpen(e.Name)
	return c.infer(e.Body)
}

func (c *Checker) inferLabelledArg(e *ast.LabelledArgExpr) types.Type {
	if e.Value != nil {
		return c.infer(e.Value)
	}
	s := c.env.Lookup(e.Label)
	if s == nil {
		c.errorfAt(e.Loc, "unbound labelled argument ~%s", e.Label)
		return c.fresh("label")
	}
	return s.Instantiate()
}

func (c *Checker) checkPerformInGo(e *ast.GoExpr) {
	var walk func(ast.Expr)
	walk = func(ex ast.Expr) {
		if ex == nil {
			return
		}
		switch ex := ex.(type) {
		case *ast.PerformExpr:
			c.errorfAt(ex.Loc, "perform is not allowed inside go body")
		case *ast.GoExpr:
			walk(ex.Expr)
		case *ast.AppExpr:
			walk(ex.Func)
			walk(ex.Arg)
		case *ast.IfExpr:
			walk(ex.Cond)
			walk(ex.ThenBranch)
			walk(ex.ElseBranch)
		case *ast.MatchExpr:
			walk(ex.Scrutinee)
			for _, a := range ex.Arms {
				walk(a.Body)
				walk(a.Guard)
			}
		case *ast.LetInExpr:
			for _, b := range ex.Bindings {
				walk(b.Body)
			}
			walk(ex.Body)
		case *ast.BeginExpr:
			for _, s := range ex.Stmts {
				walk(s)
			}
		case *ast.FunExpr:
			walk(ex.Body)
		case *ast.BinaryExpr:
			walk(ex.Left)
			walk(ex.Right)
		}
	}
	walk(e.Expr)
}

// checkMLIIfPresent parses foo.mli next to srcFile and ensures declared vals exist.
func (c *Checker) checkMLIIfPresent(mod *ast.Module, srcFile string) {
	if srcFile == "" {
		return
	}
	base := strings.TrimSuffix(srcFile, filepath.Ext(srcFile))
	mliPath := base + ".mli"
	data, err := os.ReadFile(mliPath)
	if err != nil {
		return
	}
	// Minimal .mli: treat as `sig ... end` items or bare `val` lines.
	// Reuse module-type parser by wrapping.
	src := "module type __MLI__ = sig\n" + string(data) + "\nend\n"
	mliMod, perr := parser.Parse(mliPath, []byte(src))
	if perr != nil {
		c.errorf("parsing %s: %v", mliPath, perr)
		return
	}
	var items []ast.SigItem
	for _, d := range mliMod.Decls {
		if mt, ok := d.(*ast.ModuleTypeDecl); ok {
			items = mt.Items
			break
		}
	}
	exported := make(map[string]bool)
	for _, d := range mod.Decls {
		switch d := d.(type) {
		case *ast.LetDecl:
			if d.Private {
				continue
			}
			for _, b := range d.Bindings {
				exported[b.Name] = true
			}
		case *ast.TypeDecl:
			if !d.Private {
				exported[d.Name] = true
			}
		}
	}
	for _, it := range items {
		if it.Kind == "val" && !exported[it.Name] {
			c.errorf("%s: missing export val %s (required by .mli)", mliPath, it.Name)
		}
	}
}

// mergeSiblingModules documents/stubs multi-file same-module merge.
// Full merge is optional; currently a no-op with a comment for future work.
func mergeSiblingModules(mod *ast.Module, srcFile string) {
	_ = mod
	_ = srcFile
	// STUB: when multiple sibling .goop files share `module Name`, decls
	// would be concatenated here before typechecking. Not implemented yet.
}

func (c *Checker) inferMatchEffectArms(e *ast.MatchExpr, scrutType, resultType types.Type) {
	for _, arm := range e.Arms {
		saved := c.env
		c.env = NewEnv(c.env)
		if arm.EffectHandler {
			// Pattern is the effect operation; cont is a function.
			c.checkPattern(e.Loc, arm.Pattern, scrutType)
			if arm.ContName != "" {
				a := c.fresh("'a")
				r := c.fresh("'r")
				c.env.Bind(arm.ContName, types.Mono(&types.TFun{From: a, To: r}))
			}
		} else {
			c.checkPattern(e.Loc, arm.Pattern, scrutType)
		}
		if arm.Guard != nil {
			c.unifyAt(e.Loc, c.infer(arm.Guard), types.Bool)
		}
		c.unifyAt(e.Loc, c.infer(arm.Body), resultType)
		c.env = saved
	}
}

// checkOrPattern binds variables that appear in all alternatives (approx: bind from left).
func (c *Checker) checkOrPattern(loc token.SourceLoc, p *ast.OrPattern, scrutType types.Type) {
	c.checkPattern(loc, p.Left, scrutType)
	// Right side checked in a throwaway env so we don't double-bind.
	saved := c.env
	c.env = NewEnv(saved)
	c.checkPattern(loc, p.Right, scrutType)
	c.env = saved
}
