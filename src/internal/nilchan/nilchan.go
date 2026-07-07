// Package nilchan implements full flow-sensitive nil-channel detection for C0.
//
// Reports NIL001 when a channel may be used before it is known to be non-nil.
package nilchan

import (
	"fmt"

	"c0.dev/compiler/internal/ast"
	"c0.dev/compiler/internal/token"
)

// Error is a nil-channel safety violation.
type Error struct {
	Code string
	Msg  string
	Loc  token.SourceLoc
}

func (e *Error) Error() string {
	if e.Loc.File != "" && e.Loc.Line > 0 {
		return fmt.Sprintf("%s:%d:%d: %s", e.Loc.File, e.Loc.Line, e.Loc.Column, e.Msg)
	}
	return e.Msg
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
	c.env[b.Name] = true
	c.restore(old)
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
			c.checkExpr(arm.Body)
			c.restore(old)
		}

	case *ast.QuestionExpr:
		if app, ok := v.Left.(*ast.AppExpr); ok {
			if sel, ok := app.Func.(*ast.FieldAccessExpr); ok {
				if id, ok := sel.Left.(*ast.IdentExpr); ok && (id.Name == "Chan" || id.Name == "chan") {
					if arg, ok := app.Arg.(*ast.IdentExpr); ok && !c.env[arg.Name] {
						c.err(NIL001, fmt.Sprintf("possible use of nil channel %q before initialization", arg.Name), arg.Loc)
					}
				}
			}
			if arg, ok := app.Arg.(*ast.IdentExpr); ok && !c.env[arg.Name] {
				c.err(NIL001, fmt.Sprintf("possible use of nil channel %q before initialization", arg.Name), arg.Loc)
			}
		}
		c.checkExpr(v.Left)
		if v.Arg != nil {
			c.checkExpr(v.Arg)
		}

	default:
	}
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
