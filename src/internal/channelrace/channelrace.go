package channelrace

import (
	"fmt"

	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/config"
	"goop.dev/compiler/internal/token"
)

// Error is a channel-mediated race warning/error.
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

func (e *Error) GetLoc() token.SourceLoc { return e.Loc }

// CheckWithConfig runs channel-mediated race analysis.
func CheckWithConfig(mod *ast.Module, cfg *config.Config) (errors, warnings []error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	c := &checker{
		mutableVars:     make(map[string]bool),
		accessedAfterGo: make(map[string]bool),
		pendingParent:   make(map[string]token.SourceLoc),
		cfg:             cfg,
	}
	for _, d := range mod.Decls {
		if ld, ok := d.(*ast.LetDecl); ok {
			for _, b := range ld.Bindings {
				if ld.Mutable || (len(b.Params) == 0 && isRefAlloc(b.Body)) {
					c.mutableVars[b.Name] = true
				}
			}
			for i := range ld.Bindings {
				c.checkLetBinding(&ld.Bindings[i])
			}
		}
	}
	c.flushRaces()
	return c.errors, c.warnings
}

type checker struct {
	errors          []error
	warnings        []error
	mutableVars     map[string]bool
	goSeenInSeq     bool
	inGo            bool
	accessedAfterGo map[string]bool
	pendingParent   map[string]token.SourceLoc
	cfg             *config.Config
}

func (c *checker) report(loc token.SourceLoc, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	e := &Error{Code: "LINEAR008", Msg: msg, Loc: loc}
	switch c.cfg.Check.Concurrent {
	case config.SeverityOff:
		return
	case config.SeverityWarn:
		c.warnings = append(c.warnings, e)
	default:
		c.errors = append(c.errors, e)
	}
}

func (c *checker) checkLetBinding(b *ast.LetBinding) {
	c.checkExpr(b.Body)
}

// isRefAlloc reports whether e is a `ref …` allocation (parens unwrapped).
func isRefAlloc(e ast.Expr) bool {
	for {
		switch v := e.(type) {
		case *ast.ParenExpr:
			e = v.Inner
		case *ast.RefExpr:
			return true
		default:
			return false
		}
	}
}

func (c *checker) checkExpr(e ast.Expr) {
	if e == nil {
		return
	}
	switch e := e.(type) {
	case *ast.LetInExpr:
		for _, b := range e.Bindings {
			if e.Mutable || (len(b.Params) == 0 && isRefAlloc(b.Body)) {
				c.mutableVars[b.Name] = true
			}
		}
		for i := range e.Bindings {
			c.checkLetBinding(&e.Bindings[i])
		}
		c.checkExpr(e.Body)
	case *ast.AppExpr:
		c.checkChanSend(e)
		c.checkExpr(e.Func)
		c.checkExpr(e.Arg)
	case *ast.BinaryExpr:
		if e.Op == token.LARROW {
			if id, ok := e.Left.(*ast.IdentExpr); ok && c.mutableVars[id.Name] {
				if c.goSeenInSeq && !c.inGo {
					c.accessedAfterGo[id.Name] = true
				}
			}
		}
		c.checkExpr(e.Left)
		c.checkExpr(e.Right)
	case *ast.AssignExpr:
		c.checkExpr(e.Target)
		c.checkExpr(e.Value)
	case *ast.RefExpr:
		c.checkExpr(e.Value)
	case *ast.DerefExpr:
		c.checkExpr(e.Target)
	case *ast.BeginExpr:
		for _, s := range e.Stmts {
			c.checkExpr(s)
		}
	case *ast.IfExpr:
		c.checkExpr(e.Cond)
		c.checkExpr(e.ThenBranch)
		c.checkExpr(e.ElseBranch)
	case *ast.MatchExpr:
		c.checkExpr(e.Scrutinee)
		for _, arm := range e.Arms {
			c.checkExpr(arm.Body)
		}
	case *ast.GoExpr:
		c.inGo = true
		c.checkExpr(e.Expr)
		c.inGo = false
		c.goSeenInSeq = true
	case *ast.IdentExpr:
		if c.goSeenInSeq && !c.inGo && c.mutableVars[e.Name] {
			c.accessedAfterGo[e.Name] = true
		}
	case *ast.FunExpr:
		c.checkExpr(e.Body)
	case *ast.ParenExpr:
		c.checkExpr(e.Inner)
	case *ast.PipeExpr:
		c.checkExpr(e.Left)
		c.checkExpr(e.Right)
	case *ast.SelectExpr:
		for _, cs := range e.Cases {
			c.checkExpr(cs.Recv)
			c.checkExpr(cs.Body)
		}
		if e.Default != nil {
			c.checkExpr(e.Default)
		}
	case *ast.CompExpr, *ast.RegionExpr:
		c.checkCompRegion(e)
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

func (c *checker) checkChanSend(app *ast.AppExpr) {
	_, val, loc, ok := chanSendArg(app)
	if !ok || !c.mutableVars[val] {
		return
	}
	if c.inGo {
		c.pendingParent[val] = loc
	}
}

func chanSendArg(app *ast.AppExpr) (ch, val string, loc token.SourceLoc, ok bool) {
	for cur := app; cur != nil; {
		if inner, isInner := cur.Func.(*ast.AppExpr); isInner {
			if chID, l, found := chanSendPartial(inner); found {
				if name, okID := mutableCellName(cur.Arg); okID {
					return chID, name, l, true
				}
				return chID, "", l, true
			}
		}
		if chID, l, found := chanSendPartial(cur); found {
			return chID, "", l, true
		}
		if next, okNext := cur.Func.(*ast.AppExpr); okNext {
			cur = next
			continue
		}
		break
	}
	return "", "", token.SourceLoc{}, false
}

// mutableCellName returns the ident of a ref/mutable cell, including `!x`.
func mutableCellName(e ast.Expr) (string, bool) {
	for {
		switch v := e.(type) {
		case *ast.ParenExpr:
			e = v.Inner
		case *ast.DerefExpr:
			e = v.Target
		case *ast.IdentExpr:
			return v.Name, true
		default:
			return "", false
		}
	}
}

func moduleName(e ast.Expr) (string, bool) {
	switch e := e.(type) {
	case *ast.IdentExpr:
		return e.Name, true
	case *ast.ConstructorExpr:
		return e.Name, true
	default:
		return "", false
	}
}

func chanSendPartial(app *ast.AppExpr) (ch string, loc token.SourceLoc, ok bool) {
	sel, isSel := app.Func.(*ast.FieldAccessExpr)
	if !isSel {
		return "", token.SourceLoc{}, false
	}
	mod, okMod := moduleName(sel.Left)
	if !okMod || mod != "Chan" || sel.Field != "send" {
		return "", token.SourceLoc{}, false
	}
	if id, okID := app.Arg.(*ast.IdentExpr); okID {
		return id.Name, app.Loc, true
	}
	return "", token.SourceLoc{}, false
}

func (c *checker) flushRaces() {
	for name, loc := range c.pendingParent {
		if c.accessedAfterGo[name] {
			c.report(loc, "potential channel-mediated data race: mutable variable %q sent on channel while still accessible in spawning scope", name)
		}
	}
}
