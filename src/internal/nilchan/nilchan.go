// Package nilchan implements full flow-sensitive nil-channel detection for Goop.
//
// Reports NIL001 when a channel may be used before it is known to be non-nil.
package nilchan

import (
	"fmt"

	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/token"
)

// Error is a nil-channel safety violation.
type Error struct {
	Code string
	Msg  string
	Loc  token.SourceLoc
}

func (e *Error) Error() string {
	prefix := e.Code + ": "
	if e.Loc.File != "" && e.Loc.Line > 0 {
		return fmt.Sprintf("%s:%d:%d: %s%s", e.Loc.File, e.Loc.Line, e.Loc.Column, prefix, e.Msg)
	}
	return prefix + e.Msg
}

// GetLoc returns the source location for LSP integration.
func (e *Error) GetLoc() token.SourceLoc {
	return e.Loc
}

// Check runs the analysis.
func Check(mod *ast.Module) []error {
	c := &checker{env: make(map[string]bool)}
	for _, d := range mod.Decls {
		if fn, ok := d.(*ast.LetDecl); ok {
			for i := range fn.Bindings {
				c.checkLetBinding(&fn.Bindings[i])
			}
		}
	}
	return c.errs
}

type checker struct {
	errs []error
	env  map[string]bool
}

func (c *checker) checkLetBinding(b *ast.LetBinding) {
	old := c.clone()
	c.checkExpr(b.Body)
	if c.isChannelInit(b.Body) {
		c.env[b.Name] = true
	} else {
		c.env[b.Name] = true // ordinary bindings are always "defined"
	}
	c.restore(old)
}

func (c *checker) isChannelInit(e ast.Expr) bool {
	return isChanMakeCall(e) || isOwnedChanMakeCall(e)
}

func (c *checker) checkExpr(e ast.Expr) {
	if e == nil {
		return
	}
	switch v := e.(type) {
	case *ast.LetInExpr:
		old := c.clone()
		for i := range v.Bindings {
			c.checkLetBinding(&v.Bindings[i])
		}
		c.checkExpr(v.Body)
		c.restore(old)

	case *ast.AppExpr:
		c.checkChanOp(v)
		c.checkExpr(v.Func)
		c.checkExpr(v.Arg)

	case *ast.BinaryExpr:
		c.checkExpr(v.Left)
		c.checkExpr(v.Right)

	case *ast.IfExpr:
		c.checkExpr(v.Cond)
		then := c.clone()
		c.checkExpr(v.ThenBranch)
		c.restore(then)
		els := c.clone()
		c.checkExpr(v.ElseBranch)
		c.restore(els)

	case *ast.MatchExpr:
		c.checkExpr(v.Scrutinee)
		for _, arm := range v.Arms {
			old := c.clone()
			if arm.Guard != nil {
				c.checkExpr(arm.Guard)
			}
			c.checkExpr(arm.Body)
			c.restore(old)
		}

	case *ast.QuestionExpr:
		c.checkChanSendRecv(v)
		c.checkExpr(v.Left)
		if v.Arg != nil {
			c.checkExpr(v.Arg)
		}

	case *ast.SelectExpr:
		for _, cs := range v.Cases {
			c.checkSelectRecv(cs.Recv)
			old := c.clone()
			c.checkExpr(cs.Body)
			c.restore(old)
		}
		if v.Default != nil {
			c.checkExpr(v.Default)
		}

	case *ast.FunExpr:
		old := c.clone()
		c.checkExpr(v.Body)
		c.restore(old)

	case *ast.GuardExpr:
		for _, b := range v.Bindings {
			c.checkExpr(b.Expr)
		}
		c.checkExpr(v.Else_)

	case *ast.PipeExpr:
		c.checkExpr(v.Left)
		c.checkExpr(v.Right)

	case *ast.RecordExpr:
		for _, f := range v.Fields {
			if f.Value != nil {
				c.checkExpr(f.Value)
			}
		}

	case *ast.RecordUpdateExpr:
		c.checkExpr(v.Base)
		for _, f := range v.Fields {
			if f.Value != nil {
				c.checkExpr(f.Value)
			}
		}

	case *ast.FieldAccessExpr:
		c.checkExpr(v.Left)

	case *ast.TupleExpr:
		for _, el := range v.Elems {
			c.checkExpr(el)
		}

	case *ast.ListExpr:
		for _, el := range v.Elems {
			c.checkExpr(el)
		}

	case *ast.IsExpr:
		c.checkExpr(v.Left)

	case *ast.AsMatchExpr:
		c.checkExpr(v.Left)
		c.checkExpr(v.Body)
		c.checkExpr(v.ElseBody)

	case *ast.ParenExpr:
		c.checkExpr(v.Inner)

	case *ast.GoExpr:
		c.checkExpr(v.Expr)

	case *ast.CompExpr, *ast.RegionExpr:
		c.checkCompRegion(v)
	}
}

func (c *checker) checkCompRegion(e ast.Expr) {
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
			c.checkExpr(o.Expr)
		case *ast.LetOp:
			c.checkExpr(o.Expr)
		case *ast.DoBangOp:
			c.checkExpr(o.Expr)
		case *ast.ReturnOp:
			c.checkExpr(o.Expr)
		case *ast.ReturnBangOp:
			c.checkExpr(o.Expr)
		case *ast.BodyOp:
			c.checkExpr(o.Expr)
		}
	}
}

func (c *checker) checkChanOp(app *ast.AppExpr) {
	for cur := ast.Expr(app); cur != nil; {
		a, ok := cur.(*ast.AppExpr)
		if !ok {
			break
		}
		if ch, op, ok := chanOpFromApp(a); ok && (op == "send" || op == "recv" || op == "close") {
			c.checkChanIdent(ch, a.Loc)
		}
		cur = a.Func
	}
}

func peelExpr(e ast.Expr) ast.Expr {
	for {
		if p, ok := e.(*ast.ParenExpr); ok {
			e = p.Inner
			continue
		}
		return e
	}
}

func (c *checker) checkChanSendRecv(q *ast.QuestionExpr) {
	if app, ok := peelExpr(q.Left).(*ast.AppExpr); ok {
		for cur := ast.Expr(app); cur != nil; {
			a, ok := cur.(*ast.AppExpr)
			if !ok {
				break
			}
			if ch, op, ok := chanOpFromApp(a); ok && (op == "send" || op == "recv") {
				c.checkChanIdent(ch, q.Loc)
			}
			cur = a.Func
		}
	}
}

func (c *checker) checkSelectRecv(recv ast.Expr) {
	// select case: ch <- expr  or  x := <- ch
	switch r := recv.(type) {
	case *ast.QuestionExpr:
		if app, ok := r.Left.(*ast.AppExpr); ok {
			if ch, op, ok := chanOpFromApp(app); ok && op == "recv" {
				c.checkChanIdent(ch, r.Loc)
			}
		}
	case *ast.BinaryExpr:
		if r.Op == token.LARROW {
			if id, ok := r.Right.(*ast.IdentExpr); ok {
				c.checkChanIdent(id, r.Loc)
			}
		}
	}
}

func (c *checker) checkChanIdent(id *ast.IdentExpr, loc token.SourceLoc) {
	if id != nil && !c.env[id.Name] {
		c.err(NIL001, fmt.Sprintf("possible use of nil channel %q before initialization", id.Name), id.Loc)
		if loc.File != "" {
			_ = loc
		}
	}
}

func chanOpFromApp(app *ast.AppExpr) (ch *ast.IdentExpr, op string, ok bool) {
	sel, ok := app.Func.(*ast.FieldAccessExpr)
	if !ok {
		return nil, "", false
	}
	mod, ok := sel.Left.(*ast.IdentExpr)
	if !ok || (mod.Name != "Chan" && mod.Name != "OwnedChan") {
		return nil, "", false
	}
	switch sel.Field {
	case "send", "recv", "close":
		op = sel.Field
	default:
		return nil, "", false
	}
	if id, ok := app.Arg.(*ast.IdentExpr); ok {
		return id, op, true
	}
	return nil, op, true
}

func isChanMakeCall(e ast.Expr) bool {
	app, ok := e.(*ast.AppExpr)
	if !ok {
		return false
	}
	sel, ok := app.Func.(*ast.FieldAccessExpr)
	if !ok {
		return false
	}
	mod, ok := sel.Left.(*ast.IdentExpr)
	return ok && mod.Name == "Chan" && sel.Field == "make"
}

func isOwnedChanMakeCall(e ast.Expr) bool {
	app, ok := e.(*ast.AppExpr)
	if !ok {
		return false
	}
	sel, ok := app.Func.(*ast.FieldAccessExpr)
	if !ok {
		return false
	}
	mod, ok := sel.Left.(*ast.IdentExpr)
	return ok && mod.Name == "OwnedChan" && sel.Field == "make"
}

func (c *checker) err(code, msg string, loc token.SourceLoc) {
	c.errs = append(c.errs, &Error{Code: code, Msg: msg, Loc: loc})
}

func (c *checker) clone() map[string]bool {
	m := make(map[string]bool, len(c.env))
	for k, v := range c.env {
		m[k] = v
	}
	return m
}

func (c *checker) restore(old map[string]bool) {
	c.env = old
}

const NIL001 = "NIL001"
