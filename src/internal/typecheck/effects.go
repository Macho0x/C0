package typecheck

import (
	"strings"
	"unicode"

	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/prelude"
	"goop.dev/compiler/internal/types"
)

// newtypeCtorName converts snake_case type names to PascalCase constructor names.
func newtypeCtorName(typeName string) string {
	parts := strings.Split(typeName, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		runes := []rune(p)
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, "")
}

func attachSchemeEffects(scheme *types.Scheme, effects []string) *types.Scheme {
	if scheme == nil {
		return scheme
	}
	row := &types.EffectRow{Effects: append([]string(nil), effects...)}
	return &types.Scheme{
		Vars: scheme.Vars,
		Type: setOutermostEffects(scheme.Type, row),
	}
}

func setOutermostEffects(t types.Type, row *types.EffectRow) types.Type {
	fn, ok := t.(*types.TFun)
	if !ok {
		return t
	}
	if inner, ok := fn.To.(*types.TFun); ok {
		return &types.TFun{From: fn.From, To: setOutermostEffects(inner, row), Effects: fn.Effects}
	}
	return &types.TFun{From: fn.From, To: fn.To, Effects: row}
}

func attachTypeEffects(t types.Type, row *types.EffectRow) types.Type {
	return setOutermostEffects(t, row)
}

func outermostTFun(t types.Type) *types.TFun {
	for {
		fn, ok := t.(*types.TFun)
		if !ok {
			return nil
		}
		if _, inner := fn.To.(*types.TFun); !inner {
			return fn
		}
		t = fn.To
	}
}

func effectRowFromAST(er *ast.EffectRowType) *types.EffectRow {
	if er == nil {
		return nil
	}
	row := &types.EffectRow{
		Effects: append([]string(nil), er.Effects...),
		Open:    er.Open,
	}
	if er.Rest != "" {
		row.Rest = types.Fresh(er.Rest)
	}
	return row
}

func effectSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, e := range a {
		if !hasEffectName(b, e) {
			return false
		}
	}
	return true
}

func hasEffectName(set []string, name string) bool {
	for _, e := range set {
		if e == name {
			return true
		}
	}
	return false
}

func effectSubset(have, required []string) bool {
	for _, e := range have {
		if !hasEffectName(required, e) {
			return false
		}
	}
	return true
}

func (c *Checker) finishBindingEffects(b ast.LetBinding, fnType types.Type) types.Type {
	if !c.effectInference || len(b.Params) == 0 {
		return fnType
	}
	outer := outermostTFun(types.Apply(c.sub, fnType))
	if outer == nil {
		return fnType
	}
	inferred := c.collectEffects(b.Body)
	inferred = c.unionEffects(inferred, c.effectsFromGo(b.Body))

	if b.RetEffects == nil {
		if len(inferred) > 0 {
			outer.Effects = &types.EffectRow{Effects: inferred}
		}
		return fnType
	}

	explicit := effectRowFromAST(b.RetEffects)
	if explicit == nil {
		return fnType
	}
	if explicit.Open {
		return fnType
	}
	if len(explicit.Effects) == 0 {
		if len(inferred) > 0 {
			c.errorfAt(locOf(b.Body), "UNIFY018: function declared pure `with {}` but body uses effects: %s", strings.Join(inferred, ", "))
		}
		outer.Effects = explicit
		return fnType
	}
	if !effectSubset(inferred, explicit.Effects) {
		c.errorfAt(locOf(b.Body), "UNIFY019: inferred effects {%s} exceed declared effects {%s}", strings.Join(inferred, ", "), strings.Join(explicit.Effects, ", "))
	}
	outer.Effects = explicit
	return fnType
}

func (c *Checker) unionEffects(a, b []string) []string {
	m := make(map[string]bool)
	for _, e := range a {
		m[e] = true
	}
	for _, e := range b {
		m[e] = true
	}
	out := make([]string, 0, len(m))
	for e := range m {
		out = append(out, e)
	}
	return out
}

func (c *Checker) effectsFromGo(e ast.Expr) []string {
	if ge, ok := e.(*ast.GoExpr); ok {
		_ = ge
		return []string{"async"}
	}
	var out []string
	c.walkExprForGo(e, &out)
	return out
}

func (c *Checker) walkExprForGo(e ast.Expr, out *[]string) {
	if e == nil {
		return
	}
	switch v := e.(type) {
	case *ast.GoExpr:
		*out = c.unionEffects(*out, []string{"async"})
		c.walkExprForGo(v.Expr, out)
	case *ast.LetInExpr:
		for _, b := range v.Bindings {
			c.walkExprForGo(b.Body, out)
		}
		c.walkExprForGo(v.Body, out)
	case *ast.IfExpr:
		c.walkExprForGo(v.ThenBranch, out)
		c.walkExprForGo(v.ElseBranch, out)
	case *ast.AppExpr:
		c.walkExprForGo(v.Func, out)
		c.walkExprForGo(v.Arg, out)
	case *ast.FunExpr:
		c.walkExprForGo(v.Body, out)
	case *ast.MatchExpr:
		for _, arm := range v.Arms {
			c.walkExprForGo(arm.Body, out)
		}
	case *ast.BinaryExpr:
		c.walkExprForGo(v.Left, out)
		c.walkExprForGo(v.Right, out)
	case *ast.PipeExpr:
		c.walkExprForGo(v.Left, out)
		c.walkExprForGo(v.Right, out)
	case *ast.QuestionExpr:
		c.walkExprForGo(v.Left, out)
		if v.Arg != nil {
			c.walkExprForGo(v.Arg, out)
		}
	case *ast.GuardExpr:
		for _, b := range v.Bindings {
			c.walkExprForGo(b.Expr, out)
		}
		c.walkExprForGo(v.Else_, out)
	case *ast.SelectExpr:
		for _, cs := range v.Cases {
			c.walkExprForGo(cs.Body, out)
		}
		if v.Default != nil {
			c.walkExprForGo(v.Default, out)
		}
	}
}

func (c *Checker) collectEffects(e ast.Expr) []string {
	var out []string
	c.walkEffects(e, &out)
	return out
}

func (c *Checker) walkEffects(e ast.Expr, out *[]string) {
	if e == nil {
		return
	}
	switch v := e.(type) {
	case *ast.AppExpr:
		c.addCallEffects(v, out)
		c.walkEffects(v.Func, out)
		c.walkEffects(v.Arg, out)
	case *ast.LetInExpr:
		for _, b := range v.Bindings {
			c.walkEffects(b.Body, out)
		}
		c.walkEffects(v.Body, out)
	case *ast.IfExpr:
		c.walkEffects(v.Cond, out)
		c.walkEffects(v.ThenBranch, out)
		c.walkEffects(v.ElseBranch, out)
	case *ast.MatchExpr:
		c.walkEffects(v.Scrutinee, out)
		for _, arm := range v.Arms {
			if arm.Guard != nil {
				c.walkEffects(arm.Guard, out)
			}
			c.walkEffects(arm.Body, out)
		}
	case *ast.FunExpr:
		c.walkEffects(v.Body, out)
	case *ast.BinaryExpr:
		c.walkEffects(v.Left, out)
		c.walkEffects(v.Right, out)
	case *ast.PipeExpr:
		c.walkEffects(v.Left, out)
		c.walkEffects(v.Right, out)
	case *ast.QuestionExpr:
		c.walkEffects(v.Left, out)
		if v.Arg != nil {
			c.walkEffects(v.Arg, out)
		}
	case *ast.GuardExpr:
		for _, b := range v.Bindings {
			c.walkEffects(b.Expr, out)
		}
		c.walkEffects(v.Else_, out)
	case *ast.GoExpr:
		*out = c.unionEffects(*out, []string{"async"})
		c.walkEffects(v.Expr, out)
	case *ast.SelectExpr:
		for _, cs := range v.Cases {
			c.walkEffects(cs.Recv, out)
			c.walkEffects(cs.Body, out)
		}
		if v.Default != nil {
			c.walkEffects(v.Default, out)
		}
	case *ast.RecordExpr:
		for _, f := range v.Fields {
			if f.Value != nil {
				c.walkEffects(f.Value, out)
			}
		}
	case *ast.TupleExpr:
		for _, el := range v.Elems {
			c.walkEffects(el, out)
		}
	case *ast.ListExpr:
		for _, el := range v.Elems {
			c.walkEffects(el, out)
		}
	case *ast.ParenExpr:
		c.walkEffects(v.Inner, out)
	case *ast.CompExpr, *ast.RegionExpr:
		c.walkCompEffects(v, out)
	}
}

func (c *Checker) walkCompEffects(e ast.Expr, out *[]string) {
	var ops []ast.CompOp
	switch v := e.(type) {
	case *ast.CompExpr:
		ops = v.Ops
	case *ast.RegionExpr:
		ops = v.Ops
	}
	for _, op := range ops {
		switch o := op.(type) {
		case *ast.LetBangOp:
			c.walkEffects(o.Expr, out)
		case *ast.LetOp:
			c.walkEffects(o.Expr, out)
		case *ast.DoBangOp:
			c.walkEffects(o.Expr, out)
		case *ast.ReturnOp:
			c.walkEffects(o.Expr, out)
		case *ast.ReturnBangOp:
			c.walkEffects(o.Expr, out)
		case *ast.BodyOp:
			c.walkEffects(o.Expr, out)
		}
	}
}

func (c *Checker) addCallEffects(app *ast.AppExpr, out *[]string) {
	name := calleeName(app.Func)
	if name == "" {
		return
	}
	if effs := preludeEffectsFor(name); effs != nil {
		*out = c.unionEffects(*out, effs)
		return
	}
	if s := c.env.Lookup(name); s != nil {
		if effs := effectsFromScheme(s); len(effs) > 0 {
			*out = c.unionEffects(*out, effs)
		}
	}
}

func calleeName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.IdentExpr:
		return v.Name
	case *ast.FieldAccessExpr:
		if mod, ok := v.Left.(*ast.IdentExpr); ok {
			return mod.Name + "." + v.Field
		}
	}
	return ""
}

func preludeEffectsFor(name string) []string {
	b := prelude.Default().Lookup(name)
	if b == nil || b.Effects == nil {
		return nil
	}
	return append([]string(nil), *b.Effects...)
}

func effectsFromScheme(s *types.Scheme) []string {
	if s == nil {
		return nil
	}
	fn := outermostTFun(s.Type)
	if fn == nil || fn.Effects == nil {
		return nil
	}
	return append([]string(nil), fn.Effects.Effects...)
}
