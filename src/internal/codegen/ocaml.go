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
			for _, peer := range d.RecDecls {
				if !peer.IsApp {
					out = append(out, flattenDecls(peer.Decls)...)
				}
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
	if d.TypeOnly {
		return
	}
	fields, methods := g.classMembers(d, map[string]bool{})
	name := g.goName(d.Name)
	g.emitf("type %s struct {\n", name)
	for _, f := range fields {
		ft := "interface{}"
		g.emitf("\t%s %s\n", g.goName(f.Name), ft)
	}
	g.emitf("}\n\n")
	if !d.Virtual {
		g.emitf("func New%s() *%s {\n", name, name)
		g.emitf("\to := &%s{}\n", name)
		for _, f := range fields {
			g.emitf("\to.%s = ", g.goName(f.Name))
			if f.Value != nil {
				g.emitExpr(f.Value, false)
			} else {
				g.emitf("nil")
			}
			g.emitf("\n")
		}
		for _, init := range d.Initializers {
			g.emitf("\t")
			g.emitExpr(init, true)
		}
		g.emitf("\treturn o\n}\n\n")
	}
	for _, m := range methods {
		if m.Virtual {
			continue
		}
		g.emitf("func (self *%s) %s(", name, exported(m.Name))
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

func (g *Generator) classMembers(d *ast.ClassDecl, visiting map[string]bool) ([]ast.ClassField, []ast.ClassMethod) {
	if visiting[d.Name] {
		return nil, nil
	}
	visiting[d.Name] = true
	defer delete(visiting, d.Name)
	var fields []ast.ClassField
	var methods []ast.ClassMethod
	for _, parent := range d.Inherits {
		if p := g.classes[parent]; p != nil {
			pf, pm := g.classMembers(p, visiting)
			fields = append(fields, pf...)
			for _, method := range pm {
				methods = replaceClassMethod(methods, method)
			}
		}
	}
	fields = append(fields, d.Fields...)
	for _, method := range d.Methods {
		methods = replaceClassMethod(methods, method)
	}
	return fields, methods
}

func replaceClassMethod(methods []ast.ClassMethod, method ast.ClassMethod) []ast.ClassMethod {
	for i := range methods {
		if methods[i].Name == method.Name {
			methods[i] = method
			return methods
		}
	}
	return append(methods, method)
}

func (g *Generator) emitNewExpr(e *ast.NewExpr) {
	g.emitf("New%s()", g.goName(e.Class))
}

func (g *Generator) emitObjectExpr(e *ast.ObjectExpr) {
	g.emitf("struct{")
	for i, f := range e.Fields {
		if i > 0 {
			g.emitf("; ")
		}
		g.emitf("%s interface{}", g.goName(f.Name))
	}
	for i, m := range e.Methods {
		if len(e.Fields) > 0 || i > 0 {
			g.emitf("; ")
		}
		g.emitf("%s func(", exported(m.Name))
		for i := range m.Params {
			if i > 0 {
				g.emitf(", ")
			}
			g.emitf("interface{}")
		}
		g.emitf(") interface{}")
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
	for i, m := range e.Methods {
		if len(e.Fields) > 0 || i > 0 {
			g.emitf(", ")
		}
		g.emitf("%s: func(", exported(m.Name))
		for i, p := range m.Params {
			if i > 0 {
				g.emitf(", ")
			}
			g.emitf("%s interface{}", g.goName(p.Name))
		}
		g.emitf(") interface{} { return ")
		if m.Body != nil {
			g.emitExpr(m.Body, false)
		} else {
			g.emitf("nil")
		}
		g.emitf(" }")
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
	tag, _ := op.(string)
	return __goop_eff{Tag: tag, Cont: func(x interface{}) interface{} { return x }}
}

func __goop_handle(v interface{}) interface{} {
	return v
}
`)
}

func (g *Generator) emitLazyHelpers() {
	if !g.needsLazyRuntime {
		return
	}
	g.needFmt = g.needFmt // keep field touched
	g.emitf(`
// --- lazy runtime (memoizing) ---
type __goop_lazy struct {
	once sync.Once
	v    interface{}
	f    func() interface{}
}

func __goop_lazy_make(f func() interface{}) *__goop_lazy {
	return &__goop_lazy{f: f}
}

func __goop_lazy_force(l *__goop_lazy) interface{} {
	l.once.Do(func() { l.v = l.f() })
	return l.v
}

func __goop_lazy_from_val(v interface{}) *__goop_lazy {
	return &__goop_lazy{v: v, f: func() interface{} { return v }}
}
`)
}

func (g *Generator) emitPolyvarHelpers() {
	if !g.needsPolyvarRuntime {
		return
	}
	g.emitf(`
// --- polymorphic-variant runtime ---
type __goop_polyvar struct {
	Tag string
	Arg interface{}
}
`)
}
