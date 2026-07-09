package refine

import (
	"goop.dev/compiler/internal/ast"
)

// SubstituteIdent replaces all occurrences of an identifier with an expression.
func SubstituteIdent(e ast.Expr, name string, replacement ast.Expr) ast.Expr {
	if e == nil {
		return nil
	}
	switch e := e.(type) {
	case *ast.IdentExpr:
		if e.Name == name {
			return CloneExpr(replacement)
		}
		return e

	case *ast.BinaryExpr:
		return &ast.BinaryExpr{
			Left:  SubstituteIdent(e.Left, name, replacement),
			Op:    e.Op,
			Right: SubstituteIdent(e.Right, name, replacement),
		}

	case *ast.ParenExpr:
		return &ast.ParenExpr{
			Inner: SubstituteIdent(e.Inner, name, replacement),
		}

	case *ast.AppExpr:
		return &ast.AppExpr{
			Func: SubstituteIdent(e.Func, name, replacement),
			Arg:  SubstituteIdent(e.Arg, name, replacement),
		}

	case *ast.FieldAccessExpr:
		return &ast.FieldAccessExpr{
			Left:  SubstituteIdent(e.Left, name, replacement),
			Field: e.Field,
		}
	}
	return e
}

// CloneExpr creates a shallow copy of an expression for substitution.
func CloneExpr(e ast.Expr) ast.Expr {
	if e == nil {
		return nil
	}
	switch e := e.(type) {
	case *ast.IdentExpr:
		return &ast.IdentExpr{Name: e.Name}
	case *ast.LitExpr:
		return &ast.LitExpr{Value: e.Value, Kind: e.Kind}
	case *ast.BinaryExpr:
		return &ast.BinaryExpr{Left: e.Left, Op: e.Op, Right: e.Right}
	case *ast.ParenExpr:
		return &ast.ParenExpr{Inner: e.Inner}
	}
	return e
}
