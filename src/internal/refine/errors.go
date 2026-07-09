package refine

import (
	"fmt"

	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/token"
)

const (
	REFINE001 = "REFINE001"
	REFINE002 = "REFINE002"
)

// Error is a refinement solver diagnostic.
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

func refineDisproven(rt *ast.RefinementType, site ast.Expr) error {
	loc := exprLoc(site)
	return &Error{
		Code: REFINE001,
		Msg:  fmt.Sprintf("refinement violated: cannot satisfy %s", ast.ExprString(rt.Pred)),
		Loc:  loc,
	}
}

func refineUnproven(rt *ast.RefinementType, site ast.Expr) error {
	loc := exprLoc(site)
	return &Error{
		Code: REFINE002,
		Msg:  fmt.Sprintf("could not prove refinement %s — runtime check emitted", ast.ExprString(rt.Pred)),
		Loc:  loc,
	}
}

func exprLoc(e ast.Expr) token.SourceLoc {
	switch e := e.(type) {
	case *ast.AppExpr:
		return e.Loc
	case *ast.BinaryExpr:
		return e.Loc
	case *ast.IfExpr:
		return e.Loc
	case *ast.IdentExpr:
		return e.Loc
	case *ast.LitExpr:
		return e.Loc
	default:
		return token.SourceLoc{}
	}
}
