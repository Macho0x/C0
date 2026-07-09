// Package desugar transforms match macros (is, as, guard) into ordinary
// match expressions so that the type checker and code generator do not
// need to know about them.
//
// Desugaring rules:
//
//	is:   expr is pattern
//	      → match expr with | pattern -> true | _ -> false
//
//	as:   expr as pattern -> then_expr else else_expr
//	      → match expr with | pattern -> then_expr | _ -> else_expr
//
//	guard (single):
//	      guard pattern = rhs else else_expr
//	      → match rhs with | pattern -> body | _ -> else_expr
//
//	guard (multiple):
//	      guard p1 = e1 and p2 = e2 else else_expr
//	      → match e1 with
//	        | p1 -> match e2 with | p2 -> body | _ -> else_expr
//	        | _ -> else_expr
package desugar

import (
	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/token"
)

// DesugarModule transforms all match-macro expressions in a module
// into ordinary match expressions.
func DesugarModule(mod *ast.Module) *ast.Module {
	for _, d := range mod.Decls {
		desugarDecl(d)
	}
	return mod
}

func desugarDecl(d ast.TopDecl) {
	switch d := d.(type) {
	case *ast.LetDecl:
		for i := range d.Bindings {
			d.Bindings[i].Body = DesugarExpr(d.Bindings[i].Body)
		}
	case *ast.TypeDecl:
		// Type declarations contain types, not expressions
	case *ast.ExternDecl:
		// Extern declarations contain types, not expressions
	case *ast.GolangEmbedDecl:
		// @golang embed blocks contain raw Go, not Goop expressions
	}
}

// DesugarExpr transforms a single expression, replacing match macros
// with ordinary match expressions.
func DesugarExpr(e ast.Expr) ast.Expr {
	if e == nil {
		return nil
	}
	switch e := e.(type) {
	case *ast.GuardExpr:
		return desugarGuard(e)
	case *ast.CompExpr:
		return desugarCompExpr(e)

	case *ast.IfExpr:
		e.Cond = DesugarExpr(e.Cond)
		e.ThenBranch = DesugarExpr(e.ThenBranch)
		e.ElseBranch = DesugarExpr(e.ElseBranch)
		return e

	case *ast.MatchExpr:
		e.Scrutinee = DesugarExpr(e.Scrutinee)
		for i := range e.Arms {
			e.Arms[i].Body = DesugarExpr(e.Arms[i].Body)
			if e.Arms[i].Guard != nil {
				e.Arms[i].Guard = DesugarExpr(e.Arms[i].Guard)
			}
		}
		return e

	case *ast.LetInExpr:
		for i := range e.Bindings {
			e.Bindings[i].Body = DesugarExpr(e.Bindings[i].Body)
		}
		e.Body = DesugarExpr(e.Body)
		return e

	case *ast.FunExpr:
		e.Body = DesugarExpr(e.Body)
		return e

	case *ast.AppExpr:
		e.Func = DesugarExpr(e.Func)
		e.Arg = DesugarExpr(e.Arg)
		return e

	case *ast.BinaryExpr:
		e.Left = DesugarExpr(e.Left)
		e.Right = DesugarExpr(e.Right)
		return e

	case *ast.PipeExpr:
		e.Left = DesugarExpr(e.Left)
		e.Right = DesugarExpr(e.Right)
		return e

	case *ast.QuestionExpr:
		e.Left = DesugarExpr(e.Left)
		if e.Arg != nil {
			e.Arg = DesugarExpr(e.Arg)
		}
		return e

	case *ast.RecordExpr:
		for i := range e.Fields {
			if e.Fields[i].Value != nil {
				e.Fields[i].Value = DesugarExpr(e.Fields[i].Value)
			}
		}
		return e

	case *ast.RecordUpdateExpr:
		e.Base = DesugarExpr(e.Base)
		for i := range e.Fields {
			if e.Fields[i].Value != nil {
				e.Fields[i].Value = DesugarExpr(e.Fields[i].Value)
			}
		}
		return e

	case *ast.FieldAccessExpr:
		e.Left = DesugarExpr(e.Left)
		return e

	case *ast.TupleExpr:
		for i := range e.Elems {
			e.Elems[i] = DesugarExpr(e.Elems[i])
		}
		return e

	case *ast.ListExpr:
		for i := range e.Elems {
			e.Elems[i] = DesugarExpr(e.Elems[i])
		}
		return e

	case *ast.ParenExpr:
		e.Inner = DesugarExpr(e.Inner)
		return e

	case *ast.GoExpr:
		e.Expr = DesugarExpr(e.Expr)
		return e

	case *ast.SelectExpr:
		for i := range e.Cases {
			e.Cases[i].Recv = DesugarExpr(e.Cases[i].Recv)
			e.Cases[i].Body = DesugarExpr(e.Cases[i].Body)
		}
		if e.Default != nil {
			e.Default = DesugarExpr(e.Default)
		}
		return e

	case *ast.UsingExpr:
		e.Expr = DesugarExpr(e.Expr)
		e.Body = DesugarExpr(e.Body)
		return e

	case *ast.RegionExpr:
		for i := range e.Ops {
			switch o := e.Ops[i].(type) {
			case *ast.LetBangOp:
				o.Expr = DesugarExpr(o.Expr)
			case *ast.LetOp:
				o.Expr = DesugarExpr(o.Expr)
			case *ast.DoBangOp:
				o.Expr = DesugarExpr(o.Expr)
			case *ast.ReturnOp:
				o.Expr = DesugarExpr(o.Expr)
			case *ast.ReturnBangOp:
				o.Expr = DesugarExpr(o.Expr)
			case *ast.BodyOp:
				o.Expr = DesugarExpr(o.Expr)
			}
		}
		return e

	default:
		return e
	}
}

// ---------------------------------------------------------------------------
// Desugaring rules
// ---------------------------------------------------------------------------

// expr is pattern → match expr with | pattern -> true | _ -> false
func desugarIs(e *ast.IsExpr) ast.Expr {
	return &ast.MatchExpr{
		Scrutinee: DesugarExpr(e.Left),
		Arms: []ast.MatchArm{
			{
				Pattern: e.Pattern,
				Body:    &ast.LitExpr{Value: true, Kind: token.TRUE},
			},
			{
				Pattern: &ast.WildcardPattern{},
				Body:    &ast.LitExpr{Value: false, Kind: token.FALSE},
			},
		},
	}
}

// expr as pattern -> then else elseExpr
// → match expr with | pattern -> then | _ -> elseExpr
func desugarAs(e *ast.AsMatchExpr) ast.Expr {
	return &ast.MatchExpr{
		Scrutinee: DesugarExpr(e.Left),
		Arms: []ast.MatchArm{
			{
				Pattern: e.Pattern,
				Body:    DesugarExpr(e.Body),
			},
			{
				Pattern: &ast.WildcardPattern{},
				Body:    DesugarExpr(e.ElseBody),
			},
		},
	}
}

// guard p1 = e1 and p2 = e2 ... else elseExpr
// → nested match where the innermost binding returns its bound value:
//
//	match e1 with | p1 -> match e2 with ... | p2 -> <bound value> | _ -> elseExpr
//	                | _ -> elseExpr
//
// For a single binding guard Err msg = s else "no error":
//
//	match s with | Err msg -> msg | _ -> "no error"
func desugarGuard(e *ast.GuardExpr) ast.Expr {
	if len(e.Bindings) == 0 {
		return e.Else_
	}

	// Build nested match from the inside out.
	// For bindings [p1=e1, p2=e2]:
	//   inner = match e2 with | p2 -> <bound value from p2> | _ -> else
	//   outer = match e1 with | p1 -> inner              | _ -> else
	//
	// The bound value from a constructor pattern is the pattern's payload.
	// For an identifier pattern, it's the matched value itself.

	inner := DesugarExpr(e.Else_)
	for i := len(e.Bindings) - 1; i >= 0; i-- {
		b := e.Bindings[i]
		var successBody ast.Expr
		if i == len(e.Bindings)-1 {
			// Last binding: the success body is the extracted value
			successBody = extractPatternValue(b.Pattern, b.Expr)
		} else {
			// Inner bindings chain to the outer match
			successBody = inner
		}
		inner = &ast.MatchExpr{
			Scrutinee: DesugarExpr(b.Expr),
			Arms: []ast.MatchArm{
				{
					Pattern: b.Pattern,
					Body:    successBody,
				},
				{
					Pattern: &ast.WildcardPattern{},
					Body:    DesugarExpr(e.Else_),
				},
			},
		}
	}
	return inner
}

// extractPatternValue creates an expression that extracts the value bound
// by a pattern. For a constructor pattern like `Err msg`, it returns
// `msg` (the bound variable). For an identifier pattern `x`, it returns `x`.
func extractPatternValue(p ast.Pattern, defaultExpr ast.Expr) ast.Expr {
	switch p := p.(type) {
	case *ast.ConstructorPattern:
		if p.Arg != nil {
			return extractPatternValue(p.Arg, defaultExpr)
		}
		// No payload — return unit
		return &ast.LitExpr{Value: nil, Kind: token.UNIT}
	case *ast.IdentPattern:
		return &ast.IdentExpr{Name: p.Name}
	case *ast.WildcardPattern:
		return defaultExpr
	case *ast.RecordPattern:
		// Return a record expression with the bound fields
		fields := make([]ast.RecordField, len(p.Fields))
		for i, f := range p.Fields {
			fields[i] = ast.RecordField{Name: f.Name}
		}
		return &ast.RecordExpr{Fields: fields}
	default:
		return defaultExpr
	}
}

// ---------------------------------------------------------------------------
// Computation expression desugaring
// ---------------------------------------------------------------------------

// desugarCompExpr desugars a computation expression into ordinary expressions.
//
// result { let! x = e; return f x }  →
//
//	match e with | Ok x -> f x | Error e -> Error e
//
// For chained let! bindings:
//
//	result { let! x = a; let! y = b; return x + y }  →
//	match a with
//	| Ok x -> match b with | Ok y -> Ok (x + y) | Error e -> Error e
//	| Error e -> Error e
//
// do! expr is like let! _ = expr.
// return! expr passes expr through directly.
func desugarCompExpr(e *ast.CompExpr) ast.Expr {
	switch e.Builder {
	case "result":
		return desugarResultCE(e)
	case "async":
		return desugarAsyncCE(e)
	case "region":
		return desugarRegionCE(e)
	default:
		// Unknown builder: leave as-is (will be caught by codegen)
		return e
	}
}

func desugarRegionCE(e *ast.CompExpr) ast.Expr {
	// Transform `region { ops }` into `RegionExpr { ops }` with
	// desugared sub-expressions.
	ops := make([]ast.CompOp, len(e.Ops))
	for i, op := range e.Ops {
		switch o := op.(type) {
		case *ast.LetBangOp:
			ops[i] = &ast.LetBangOp{Pattern: o.Pattern, Expr: DesugarExpr(o.Expr)}
		case *ast.LetOp:
			ops[i] = &ast.LetOp{Pattern: o.Pattern, Expr: DesugarExpr(o.Expr)}
		case *ast.DoBangOp:
			ops[i] = &ast.DoBangOp{Expr: DesugarExpr(o.Expr)}
		case *ast.ReturnOp:
			ops[i] = &ast.ReturnOp{Expr: DesugarExpr(o.Expr)}
		case *ast.ReturnBangOp:
			ops[i] = &ast.ReturnBangOp{Expr: DesugarExpr(o.Expr)}
		case *ast.BodyOp:
			ops[i] = &ast.BodyOp{Expr: DesugarExpr(o.Expr)}
		}
	}
	return &ast.RegionExpr{Ops: ops}
}

func desugarResultCE(e *ast.CompExpr) ast.Expr {
	if len(e.Ops) == 0 {
		return &ast.LitExpr{Value: nil, Kind: token.UNIT}
	}

	// Build nested match expressions from the last op to the first.
	var result ast.Expr
	for i := len(e.Ops) - 1; i >= 0; i-- {
		op := e.Ops[i]
		switch o := op.(type) {
		case *ast.LetBangOp:
			// match <expr> with | Ok <pat> -> <rest> | Error e -> Error e
			inner := result
			if inner == nil {
				inner = &ast.LitExpr{Value: nil, Kind: token.UNIT}
			}
			okPat := &ast.ConstructorPattern{Name: "Ok", Arg: o.Pattern}
			okArm := ast.MatchArm{Pattern: okPat, Body: inner}
			errArm := ast.MatchArm{
				Pattern: &ast.ConstructorPattern{Name: "Error", Arg: &ast.IdentPattern{Name: "_e"}},
				Body:    &ast.ConstructorExpr{Name: "Error", Arg: &ast.IdentExpr{Name: "_e"}},
			}
			result = &ast.MatchExpr{
				Scrutinee: DesugarExpr(o.Expr),
				Arms:      []ast.MatchArm{okArm, errArm},
			}

		case *ast.ReturnOp:
			result = &ast.ConstructorExpr{Name: "Ok", Arg: DesugarExpr(o.Expr)}

		case *ast.ReturnBangOp:
			result = DesugarExpr(o.Expr)

		case *ast.DoBangOp:
			inner := result
			if inner == nil {
				inner = &ast.LitExpr{Value: nil, Kind: token.UNIT}
			}
			okArm := ast.MatchArm{Pattern: &ast.WildcardPattern{}, Body: inner}
			errArm := ast.MatchArm{
				Pattern: &ast.ConstructorPattern{Name: "Error", Arg: &ast.IdentPattern{Name: "_e"}},
				Body:    &ast.ConstructorExpr{Name: "Error", Arg: &ast.IdentExpr{Name: "_e"}},
			}
			result = &ast.MatchExpr{
				Scrutinee: DesugarExpr(o.Expr),
				Arms:      []ast.MatchArm{okArm, errArm},
			}

		case *ast.LetOp:
			inner := result
			if inner == nil {
				inner = &ast.LitExpr{Value: nil, Kind: token.UNIT}
			}
			bindingName := "x"
			if ip, ok := o.Pattern.(*ast.IdentPattern); ok {
				bindingName = ip.Name
			}
			result = &ast.LetInExpr{
				Bindings: []ast.LetBinding{{
					Name: bindingName,
					Body: DesugarExpr(o.Expr),
				}},
				Body: inner,
			}

		case *ast.BodyOp:
			result = DesugarExpr(o.Expr)
		}
	}
	return result
}

func desugarAsyncCE(e *ast.CompExpr) ast.Expr {
	var result ast.Expr
	for i := len(e.Ops) - 1; i >= 0; i-- {
		op := e.Ops[i]
		switch o := op.(type) {
		case *ast.LetBangOp:
			inner := result
			if inner == nil {
				inner = unitExpr()
			}
			bindingName := patternName(o.Pattern)
			result = letIn(bindingName, asyncRecv(DesugarExpr(o.Expr)), inner)
		case *ast.ReturnOp:
			result = DesugarExpr(o.Expr)
		case *ast.ReturnBangOp:
			result = DesugarExpr(o.Expr)
		case *ast.DoBangOp:
			inner := result
			if inner == nil {
				inner = unitExpr()
			}
			result = letIn("_", asyncRecv(DesugarExpr(o.Expr)), inner)
		case *ast.LetOp:
			inner := result
			if inner == nil {
				inner = unitExpr()
			}
			result = letIn(patternName(o.Pattern), DesugarExpr(o.Expr), inner)
		case *ast.BodyOp:
			result = DesugarExpr(o.Expr)
		}
	}
	if result == nil {
		result = unitExpr()
	}
	return wrapAsyncFuture(result)
}

func unitExpr() ast.Expr {
	return &ast.LitExpr{Value: nil, Kind: token.UNIT}
}

func patternName(p ast.Pattern) string {
	if ip, ok := p.(*ast.IdentPattern); ok {
		return ip.Name
	}
	return "x"
}

func letIn(name string, rhs ast.Expr, body ast.Expr) ast.Expr {
	return &ast.LetInExpr{
		Bindings: []ast.LetBinding{{Name: name, Body: rhs}},
		Body:     body,
	}
}

func asyncRecv(ch ast.Expr) ast.Expr {
	return &ast.AppExpr{
		Func: &ast.IdentExpr{Name: "Chan.recv"},
		Arg:  ch,
	}
}

func wrapAsyncFuture(value ast.Expr) ast.Expr {
	makeCh := &ast.AppExpr{
		Func: &ast.IdentExpr{Name: "Chan.make"},
		Arg:  unitExpr(),
	}
	sendStmt := &ast.AppExpr{
		Func: &ast.AppExpr{
			Func: &ast.IdentExpr{Name: "Chan.send"},
			Arg:  &ast.IdentExpr{Name: "__async_ch"},
		},
		Arg: value,
	}
	goBody := &ast.FunExpr{
		Params: []ast.Param{{Name: ""}}, // fun () -> …
		Body:   sendStmt,
	}
	return &ast.LetInExpr{
		Bindings: []ast.LetBinding{
			{Name: "__async_ch", Body: makeCh},
			{Name: "__async_go", Body: &ast.GoExpr{Expr: goBody}},
		},
		Body: &ast.IdentExpr{Name: "__async_ch"},
	}
}

var g string // dummy for LetOp binding name
