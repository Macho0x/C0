package refine

import (
	"testing"

	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/token"
)

// Helper functions to build AST expressions for testing.
func intLit(v int64) ast.Expr {
	return &ast.LitExpr{Value: v, Kind: token.INT}
}

func ident(name string) ast.Expr {
	return &ast.IdentExpr{Name: name}
}

func binOp(left ast.Expr, op token.TokenType, right ast.Expr) ast.Expr {
	return &ast.BinaryExpr{Left: left, Op: op, Right: right}
}

func notExpr(e ast.Expr) ast.Expr {
	// not e is not a separate AST node — it's a BinaryExpr with OP as NOT?
	// In Goop, `not` is a keyword but in AST it may be represented differently.
	// For now, skip `not` tests and focus on directly checkable comparisons.
	return e
}

func TestCheck_DirectMatch(t *testing.T) {
	// x > 0 with constraint x > 0 → Proven
	pred := binOp(ident("x"), token.GT, intLit(0))
	constraints := []ast.Expr{
		binOp(ident("x"), token.GT, intLit(0)),
	}
	if got := Check(pred, constraints); got != Proven {
		t.Errorf("x > 0 with constraint x > 0: got %v, want Proven", got)
	}
}

func TestCheck_StrongerConstraint(t *testing.T) {
	// x > 0 with constraint x > 5 → Proven (5 > 0)
	pred := binOp(ident("x"), token.GT, intLit(0))
	constraints := []ast.Expr{
		binOp(ident("x"), token.GT, intLit(5)),
	}
	if got := Check(pred, constraints); got != Proven {
		t.Errorf("x > 0 with constraint x > 5: got %v, want Proven", got)
	}
}

func TestCheck_NoConstraint(t *testing.T) {
	// x > 0 with no constraints → Unknown
	pred := binOp(ident("x"), token.GT, intLit(0))
	if got := Check(pred, nil); got != Unknown {
		t.Errorf("x > 0 with no constraints: got %v, want Unknown", got)
	}
}

func TestCheck_NeqWithEqConstraint(t *testing.T) {
	// x <> 0 with constraint x = 5 → Proven
	pred := binOp(ident("x"), token.DIAMOND, intLit(0))
	constraints := []ast.Expr{
		binOp(ident("x"), token.EQEQ, intLit(5)),
	}
	if got := Check(pred, constraints); got != Proven {
		t.Errorf("x <> 0 with x = 5: got %v, want Proven", got)
	}
}

func TestCheck_NeqWithZeroConstraint(t *testing.T) {
	// x <> 0 with constraint x = 0 → Disproven
	pred := binOp(ident("x"), token.DIAMOND, intLit(0))
	constraints := []ast.Expr{
		binOp(ident("x"), token.EQEQ, intLit(0)),
	}
	if got := Check(pred, constraints); got != Disproven {
		t.Errorf("x <> 0 with x = 0: got %v, want Disproven", got)
	}
}

func TestCheck_AndWithConstraints(t *testing.T) {
	// x > 0 && x < 100 with constraints x > 5 and x < 50 → Proven
	pred := binOp(
		binOp(ident("x"), token.GT, intLit(0)),
		token.AMPAMP,
		binOp(ident("x"), token.LT, intLit(100)),
	)
	constraints := []ast.Expr{
		binOp(ident("x"), token.GT, intLit(5)),
		binOp(ident("x"), token.LT, intLit(50)),
	}
	if got := Check(pred, constraints); got != Proven {
		t.Errorf("x > 0 && x < 100 with x > 5, x < 50: got %v, want Proven", got)
	}
}

func TestCheck_OrWithConstraint(t *testing.T) {
	// x > 0 || y > 0 with constraint x > 5 → Proven
	pred := binOp(
		binOp(ident("x"), token.GT, intLit(0)),
		token.PIPEPIPE,
		binOp(ident("y"), token.GT, intLit(0)),
	)
	constraints := []ast.Expr{
		binOp(ident("x"), token.GT, intLit(5)),
	}
	if got := Check(pred, constraints); got != Proven {
		t.Errorf("x > 0 || y > 0 with x > 5: got %v, want Proven", got)
	}
}

func TestCheck_Disproven(t *testing.T) {
	// x > 0 with constraint x < -1 → Disproven
	pred := binOp(ident("x"), token.GT, intLit(0))
	constraints := []ast.Expr{
		binOp(ident("x"), token.LT, intLit(-1)),
	}
	if got := Check(pred, constraints); got != Disproven {
		t.Errorf("x > 0 with x < -1: got %v, want Disproven", got)
	}
}

func TestCheck_GeqProven(t *testing.T) {
	// x >= 5 with constraint x > 5 → Proven
	pred := binOp(ident("x"), token.GEQ, intLit(5))
	constraints := []ast.Expr{
		binOp(ident("x"), token.GT, intLit(5)),
	}
	if got := Check(pred, constraints); got != Proven {
		t.Errorf("x >= 5 with x > 5: got %v, want Proven", got)
	}
}

func TestCheck_LeqProven(t *testing.T) {
	// x <= 5 with constraint x < 5 → Proven
	pred := binOp(ident("x"), token.LEQ, intLit(5))
	constraints := []ast.Expr{
		binOp(ident("x"), token.LT, intLit(5)),
	}
	if got := Check(pred, constraints); got != Proven {
		t.Errorf("x <= 5 with x < 5: got %v, want Proven", got)
	}
}

func TestCheck_NeqProvenFromGt(t *testing.T) {
	// x <> 0 with constraint x > 0 → Proven
	pred := binOp(ident("x"), token.DIAMOND, intLit(0))
	constraints := []ast.Expr{
		binOp(ident("x"), token.GT, intLit(0)),
	}
	if got := Check(pred, constraints); got != Proven {
		t.Errorf("x <> 0 with x > 0: got %v, want Proven", got)
	}
}

func TestCheck_NeqProvenFromLt(t *testing.T) {
	// x <> 0 with constraint x < 0 → Proven
	pred := binOp(ident("x"), token.DIAMOND, intLit(0))
	constraints := []ast.Expr{
		binOp(ident("x"), token.LT, intLit(0)),
	}
	if got := Check(pred, constraints); got != Proven {
		t.Errorf("x <> 0 with x < 0: got %v, want Proven", got)
	}
}

func TestCheck_UnrelatedConstraint(t *testing.T) {
	// x > 0 with constraint y > 5 → Unknown (no info about x)
	pred := binOp(ident("x"), token.GT, intLit(0))
	constraints := []ast.Expr{
		binOp(ident("y"), token.GT, intLit(5)),
	}
	if got := Check(pred, constraints); got != Unknown {
		t.Errorf("x > 0 with y > 5: got %v, want Unknown", got)
	}
}

func TestCheck_ArithmeticInPredicate(t *testing.T) {
	// x + 1 > 0 with constraint x > -1 → Proven
	pred := binOp(
		binOp(ident("x"), token.PLUS, intLit(1)),
		token.GT,
		intLit(0),
	)
	constraints := []ast.Expr{
		binOp(ident("x"), token.GT, intLit(-1)),
	}
	if got := Check(pred, constraints); got != Proven {
		t.Errorf("x + 1 > 0 with x > -1: got %v, want Proven", got)
	}
}

func TestCheck_EqeqProven(t *testing.T) {
	// x == 5 with constraint x == 5 → Proven
	pred := binOp(ident("x"), token.EQEQ, intLit(5))
	constraints := []ast.Expr{
		binOp(ident("x"), token.EQEQ, intLit(5)),
	}
	if got := Check(pred, constraints); got != Proven {
		t.Errorf("x == 5 with x == 5: got %v, want Proven", got)
	}
}

func TestCheck_EqeqDisproven(t *testing.T) {
	// x == 5 with constraint x > 5 → Disproven
	pred := binOp(ident("x"), token.EQEQ, intLit(5))
	constraints := []ast.Expr{
		binOp(ident("x"), token.GT, intLit(5)),
	}
	if got := Check(pred, constraints); got != Disproven {
		t.Errorf("x == 5 with x > 5: got %v, want Disproven", got)
	}
}

func TestCheck_CombinedConstraints(t *testing.T) {
	// x > 5 && x < 15 with constraints x > 10 and x < 20 → Unknown (x=11 satisfies all but interval too wide)
	pred := binOp(
		binOp(ident("x"), token.GT, intLit(5)),
		token.AMPAMP,
		binOp(ident("x"), token.LT, intLit(15)),
	)
	constraints := []ast.Expr{
		binOp(ident("x"), token.GT, intLit(10)),
		binOp(ident("x"), token.LT, intLit(20)),
	}
	if got := Check(pred, constraints); got != Unknown {
		t.Errorf("x > 5 && x < 15 with x > 10, x < 20: got %v, want Unknown", got)
	}
}

func TestCheck_LiteralPred(t *testing.T) {
	// true predicate → Proven
	pred := &ast.LitExpr{Value: true, Kind: token.TRUE}
	if got := Check(pred, nil); got != Proven {
		t.Errorf("true: got %v, want Proven", got)
	}
}

func TestCheck_ReversedComparison(t *testing.T) {
	// x > 0 with constraint 0 < x → Proven (reversed comparison)
	pred := binOp(ident("x"), token.GT, intLit(0))
	constraints := []ast.Expr{
		binOp(intLit(0), token.LT, ident("x")),
	}
	if got := Check(pred, constraints); got != Proven {
		t.Errorf("x > 0 with 0 < x: got %v, want Proven", got)
	}
}
