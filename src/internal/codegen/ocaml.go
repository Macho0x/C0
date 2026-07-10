package codegen

import (
	"goop.dev/compiler/internal/ast"
)

// flattenDecls expands nested modules / includes into a flat decl list for emission.
func flattenDecls(decls []ast.TopDecl) []ast.TopDecl {
	var out []ast.TopDecl
	for _, d := range decls {
		switch d := d.(type) {
		case *ast.NestedModuleDecl:
			if !d.IsApp {
				out = append(out, flattenDecls(d.Decls)...)
			}
		case *ast.IncludeDecl, *ast.OpenModuleDecl, *ast.ModuleTypeDecl:
			// no codegen
		case *ast.ClassDecl:
			out = append(out, d)
		default:
			out = append(out, d)
		}
	}
	return out
}

func (g *Generator) emitClassDecl(d *ast.ClassDecl) {
	name := g.goName(d.Name)
	g.emitf("type %s struct {\n", name)
	for _, f := range d.Fields {
		ft := "interface{}"
		g.emitf("\t%s %s\n", g.goName(f.Name), ft)
	}
	g.emitf("}\n\n")
	g.emitf("func New%s() *%s {\n", name, name)
	g.emitf("\to := &%s{}\n", name)
	for _, f := range d.Fields {
		g.emitf("\to.%s = ", g.goName(f.Name))
		if f.Value != nil {
			g.emitExpr(f.Value, false)
		} else {
			g.emitf("nil")
		}
		g.emitf("\n")
	}
	g.emitf("\treturn o\n}\n\n")
	for _, m := range d.Methods {
		g.emitf("func (self *%s) %s(", name, g.goName(m.Name))
		for i, p := range m.Params {
			if i > 0 {
				g.emitf(", ")
			}
			g.emitf("%s interface{}", g.goName(p.Name))
		}
		g.emitf(") interface{} {\n")
		g.emitf("\treturn ")
		g.emitExpr(m.Body, false)
		g.emitf("\n}\n\n")
	}
}

func (g *Generator) emitNewExpr(e *ast.NewExpr) {
	g.emitf("New%s()", g.goName(e.Class))
}

func (g *Generator) emitObjectExpr(e *ast.ObjectExpr) {
	// Anonymous object → struct literal with method closures omitted (fields only).
	g.emitf("struct{")
	for i, f := range e.Fields {
		if i > 0 {
			g.emitf("; ")
		}
		g.emitf("%s interface{}", g.goName(f.Name))
	}
	g.emitf("}{")
	for i, f := range e.Fields {
		if i > 0 {
			g.emitf(", ")
		}
		g.emitf("%s: ", g.goName(f.Name))
		if f.Value != nil {
			g.emitExpr(f.Value, false)
		} else {
			g.emitf("nil")
		}
	}
	g.emitf("}")
}

func (g *Generator) emitLabelledArg(e *ast.LabelledArgExpr) {
	if e.Value != nil {
		g.emitExpr(e.Value, false)
		return
	}
	g.emitf("%s", g.goName(e.Label))
}

func (g *Generator) emitEffectHelpers() {
	if !g.needsEffectRuntime {
		return
	}
	g.emitf(`
// --- effect CPS runtime (minimal) ---
type __goop_eff struct {
	Tag  string
	Arg  interface{}
	Cont func(interface{}) interface{}
}

func __goop_perform(op interface{}) interface{} {
	return __goop_eff{Tag: "perform", Arg: op, Cont: func(x interface{}) interface{} { return x }}
}

func __goop_handle(v interface{}) interface{} {
	return v
}
`)
}
