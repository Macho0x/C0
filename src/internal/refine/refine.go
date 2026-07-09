// Package refine provides a built-in constraint solver for compile-time
// refinement checking. It uses interval analysis and boolean decomposition
// to prove or disprove simple integer arithmetic predicates given a set of
// known path constraints.
//
// Unproven refinements fall back to runtime panic guards (existing behaviour).
package refine

import (
	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/token"
	"fmt"
	"math"
)

// Result indicates whether a refinement was proven, disproven, or unknown.
type Result int

const (
	Proven    Result = iota // refinement holds - skip runtime check
	Disproven               // refinement fails - compile error
	Unknown                 // cannot determine - emit runtime check
)

// ProvenSites is a set of AST call-site nodes whose refinements have been
// proven at compile time. Codegen uses this to skip emitting runtime guards.
type ProvenSites map[ast.Expr]bool

// Check verifies that a refinement predicate holds given a set of known constraints.
// Returns Proven, Disproven, or Unknown.
func Check(pred ast.Expr, constraints []ast.Expr) Result {
	ivals := make(map[string]ivl)
	neqs := make(map[string]map[int64]bool)

	for _, c := range constraints {
		extractInterval(c, ivals)
		extractNeq(c, neqs)
	}

	// Direct match: if the predicate is syntactically identical to a constraint, it's Proven.
	for _, c := range constraints {
		if exprEqual(pred, c) {
			return Proven
		}
	}

	if r := constraintImplies(constraints, pred); r != Unknown {
		return r
	}

	return checkRec(pred, ivals, neqs)
}

func checkRec(e ast.Expr, ivals map[string]ivl, neqs map[string]map[int64]bool) Result {
	switch e := e.(type) {
	case *ast.BinaryExpr:
		switch e.Op {
		case token.AMPAMP: // a && b
			left := checkRec(e.Left, ivals, neqs)
			if left == Disproven {
				return Disproven
			}
			right := checkRec(e.Right, ivals, neqs)
			if left == Proven && right == Proven {
				return Proven
			}
			if left == Disproven || right == Disproven {
				return Disproven
			}
			return Unknown

		case token.PIPEPIPE: // a || b
			left := checkRec(e.Left, ivals, neqs)
			if left == Proven {
				return Proven
			}
			right := checkRec(e.Right, ivals, neqs)
			if right == Proven {
				return Proven
			}
			if left == Disproven && right == Disproven {
				return Disproven
			}
			return Unknown

		case token.DIAMOND, token.NEQ: // x <> y, x != y
			return checkInequality(e.Left, e.Right, ivals, neqs)

		case token.EQEQ, token.EQUALS: // x == y, x = y
			return checkEquality(e.Left, e.Right, ivals, neqs)

		case token.LT, token.GT, token.LEQ, token.GEQ:
			return checkComparison(e.Left, e.Op, e.Right, ivals)

		case token.PLUS, token.MINUS, token.STAR:
			return checkArithmeticPred(e, ivals)

		default:
			return Unknown
		}

	case *ast.ParenExpr:
		return checkRec(e.Inner, ivals, neqs)

	case *ast.IdentExpr:
		if v, ok := ivals[e.Name]; ok {
			if v.lo > 0 || v.hi < 0 {
				return Proven
			}
			if v.lo == 0 && v.hi == 0 {
				return Disproven
			}
		}
		return Unknown

	case *ast.LitExpr:
		if b, ok := e.Value.(bool); ok {
			if b {
				return Proven
			}
			return Disproven
		}
		if v, ok := int64FromValue(e.Value); ok {
			if v != 0 {
				return Proven
			}
			return Disproven
		}
		return Unknown

	default:
		return Unknown
	}
}

// --- interval analysis ---

type ivl struct {
	lo, hi int64
	set    bool
}

func (i ivl) valid() bool { return i.set }

func (i ivl) constrain(op token.TokenType, rhs int64) ivl {
	switch op {
	case token.LT:
		hi := rhs - 1
		if hi < i.hi {
			i.hi = hi
		}
	case token.LEQ:
		if rhs < i.hi {
			i.hi = rhs
		}
	case token.GT:
		lo := rhs + 1
		if lo > i.lo {
			i.lo = lo
		}
	case token.GEQ:
		if rhs > i.lo {
			i.lo = rhs
		}
	case token.EQEQ, token.EQUALS:
		if rhs > i.lo {
			i.lo = rhs
		}
		if rhs < i.hi {
			i.hi = rhs
		}
	}
	return i
}

func unbounded() ivl { return ivl{math.MinInt64, math.MaxInt64, true} }

// --- constraint extraction ---

func extractInterval(e ast.Expr, ivals map[string]ivl) {
	bin, ok := e.(*ast.BinaryExpr)
	if !ok {
		return
	}
	switch bin.Op {
	case token.LT, token.GT, token.LEQ, token.GEQ, token.EQEQ, token.EQUALS:
		if ident, ok := bin.Left.(*ast.IdentExpr); ok {
			if v, ok := int64FromExpr(bin.Right); ok {
				cur := ivals[ident.Name]
				if !cur.valid() {
					cur = unbounded()
				}
				cur = cur.constrain(bin.Op, v)
				ivals[ident.Name] = cur
				return
			}
		}
		if ident, ok := bin.Right.(*ast.IdentExpr); ok {
			if v, ok := int64FromExpr(bin.Left); ok {
				cur := ivals[ident.Name]
				if !cur.valid() {
					cur = unbounded()
				}
				cur = cur.constrain(reverseOp(bin.Op), v)
				ivals[ident.Name] = cur
				return
			}
		}
		if add, ok := bin.Left.(*ast.BinaryExpr); ok && (add.Op == token.PLUS || add.Op == token.MINUS) {
			if ident, ok := add.Left.(*ast.IdentExpr); ok {
				if delta, ok := int64FromExpr(add.Right); ok {
					if rhs, ok := int64FromExpr(bin.Right); ok {
						if add.Op == token.MINUS {
							delta = -delta
						}
						cur := ivals[ident.Name]
						if !cur.valid() {
							cur = unbounded()
						}
						cur = cur.constrain(bin.Op, rhs-delta)
						ivals[ident.Name] = cur
						return
					}
				}
			}
		}
	case token.AMPAMP:
		extractInterval(bin.Left, ivals)
		extractInterval(bin.Right, ivals)
	}
}

func extractNeq(e ast.Expr, neqs map[string]map[int64]bool) {
	bin, ok := e.(*ast.BinaryExpr)
	if !ok {
		return
	}
	if bin.Op != token.DIAMOND && bin.Op != token.NEQ {
		return
	}
	if ident, ok := bin.Left.(*ast.IdentExpr); ok {
		if v, ok := int64FromExpr(bin.Right); ok {
			if neqs[ident.Name] == nil {
				neqs[ident.Name] = make(map[int64]bool)
			}
			neqs[ident.Name][v] = true
		}
	}
}

func reverseOp(op token.TokenType) token.TokenType {
	switch op {
	case token.LT:
		return token.GT
	case token.GT:
		return token.LT
	case token.LEQ:
		return token.GEQ
	case token.GEQ:
		return token.LEQ
	default:
		return op
	}
}

// --- comparison helpers ---

func checkComparison(left ast.Expr, op token.TokenType, right ast.Expr, ivals map[string]ivl) Result {
	if lit, ok := int64FromExpr(right); ok {
		if iv, ok := evalExprInterval(left, ivals); ok {
			return checkIntervalAgainstOp(iv, op, lit)
		}
	}
	if lit, ok := int64FromExpr(left); ok {
		if iv, ok := evalExprInterval(right, ivals); ok {
			return checkIntervalAgainstOp(iv, reverseOp(op), lit)
		}
	}
	if ident, ok := left.(*ast.IdentExpr); ok {
		if rhs, ok := int64FromExpr(right); ok {
			if iv, ok := ivals[ident.Name]; ok && iv.valid() {
				return checkIntervalAgainstOp(iv, op, rhs)
			}
		}
	}
	if ident, ok := right.(*ast.IdentExpr); ok {
		if lhs, ok := int64FromExpr(left); ok {
			if iv, ok := ivals[ident.Name]; ok && iv.valid() {
				return checkIntervalAgainstOp(iv, reverseOp(op), lhs)
			}
		}
	}
	if add, ok := left.(*ast.BinaryExpr); ok && (add.Op == token.PLUS || add.Op == token.MINUS) {
		if ident, ok := add.Left.(*ast.IdentExpr); ok {
			if delta, ok := int64FromExpr(add.Right); ok {
				if rhs, ok := int64FromExpr(right); ok {
					if iv, ok := ivals[ident.Name]; ok && iv.valid() {
						if add.Op == token.MINUS {
							delta = -delta
						}
						return checkIntervalAgainstOp(iv, op, rhs-delta)
					}
				}
			}
		}
	}
	return Unknown
}

func checkIntervalAgainstOp(iv ivl, op token.TokenType, rhs int64) Result {
	switch op {
	case token.LT:
		if iv.hi < rhs {
			return Proven
		}
		if iv.lo >= rhs {
			return Disproven
		}
		return Unknown
	case token.GT:
		if iv.lo > rhs {
			return Proven
		}
		if iv.hi <= rhs {
			return Disproven
		}
		return Unknown
	case token.LEQ:
		if iv.hi <= rhs {
			return Proven
		}
		if iv.lo > rhs {
			return Disproven
		}
		return Unknown
	case token.GEQ:
		if iv.lo >= rhs {
			return Proven
		}
		if iv.hi < rhs {
			return Disproven
		}
		return Unknown
	case token.EQEQ, token.EQUALS:
		if iv.lo == rhs && iv.hi == rhs {
			return Proven
		}
		if iv.lo > rhs || iv.hi < rhs {
			return Disproven
		}
		return Unknown
	default:
		return Unknown
	}
}

func checkInequality(left, right ast.Expr, ivals map[string]ivl, neqs map[string]map[int64]bool) Result {
	if lit, ok := int64FromExpr(right); ok {
		if iv, ok := evalExprInterval(left, ivals); ok {
			if iv.lo > lit || iv.hi < lit {
				return Proven
			}
			if iv.lo == lit && iv.hi == lit {
				return Disproven
			}
		}
	}
	if ident, ok := left.(*ast.IdentExpr); ok {
		if rhs, ok := int64FromExpr(right); ok {
			if iv, ok := ivals[ident.Name]; ok && iv.valid() {
				if iv.lo > rhs || iv.hi < rhs {
					return Proven
				}
				if iv.lo == rhs && iv.hi == rhs {
					return Disproven
				}
			}
			if neqVals, ok := neqs[ident.Name]; ok && neqVals[rhs] {
				return Proven
			}
		}
	}
	if ident, ok := right.(*ast.IdentExpr); ok {
		if lhs, ok := int64FromExpr(left); ok {
			if iv, ok := ivals[ident.Name]; ok && iv.valid() {
				if iv.lo > lhs || iv.hi < lhs {
					return Proven
				}
				if iv.lo == lhs && iv.hi == lhs {
					return Disproven
				}
			}
			if neqVals, ok := neqs[ident.Name]; ok && neqVals[lhs] {
				return Proven
			}
		}
	}
	return Unknown
}

func checkEquality(left, right ast.Expr, ivals map[string]ivl, neqs map[string]map[int64]bool) Result {
	if ident, ok := left.(*ast.IdentExpr); ok {
		if rhs, ok := int64FromExpr(right); ok {
			if iv, ok := ivals[ident.Name]; ok && iv.valid() {
				if iv.lo == rhs && iv.hi == rhs {
					return Proven
				}
				if iv.lo > rhs || iv.hi < rhs {
					return Disproven
				}
			}
			if neqVals, ok := neqs[ident.Name]; ok && neqVals[rhs] {
				return Disproven
			}
		}
	}
	return Unknown
}

func constraintImplies(constraints []ast.Expr, pred ast.Expr) Result {
	predBin, ok := pred.(*ast.BinaryExpr)
	if !ok {
		return Unknown
	}
	for _, c := range constraints {
		cBin, ok := c.(*ast.BinaryExpr)
		if !ok {
			continue
		}
		// a > b  implies  (a - b) > 0
		if predBin.Op == token.GT {
			if z, ok := int64FromExpr(predBin.Right); ok && z == 0 {
				if sub, ok := predBin.Left.(*ast.BinaryExpr); ok && sub.Op == token.MINUS {
					if ai, ok := sub.Left.(*ast.IdentExpr); ok {
						if bi, ok := sub.Right.(*ast.IdentExpr); ok {
							if cBin.Op == token.GT && identEqual(cBin.Left, ai) && identEqual(cBin.Right, bi) {
								return Proven
							}
						}
					}
				}
			}
		}
	}
	return Unknown
}

func identEqual(e ast.Expr, id *ast.IdentExpr) bool {
	ie, ok := e.(*ast.IdentExpr)
	return ok && ie.Name == id.Name
}

func checkArithmeticPred(e *ast.BinaryExpr, ivals map[string]ivl) Result {
	iv, ok := evalExprInterval(e, ivals)
	if !ok {
		return Unknown
	}
	if iv.lo > 0 || iv.hi < 0 {
		return Proven
	}
	if iv.lo == 0 && iv.hi == 0 {
		return Disproven
	}
	return Unknown
}

// evalExprInterval returns a sound interval for simple arithmetic expressions.
func evalExprInterval(e ast.Expr, ivals map[string]ivl) (ivl, bool) {
	switch e := e.(type) {
	case *ast.IdentExpr:
		if iv, ok := ivals[e.Name]; ok && iv.valid() {
			return iv, true
		}
		return unbounded(), true

	case *ast.LitExpr:
		if v, ok := int64FromValue(e.Value); ok {
			return ivl{lo: v, hi: v, set: true}, true
		}
		return ivl{}, false

	case *ast.ParenExpr:
		return evalExprInterval(e.Inner, ivals)

	case *ast.BinaryExpr:
		left, okL := evalExprInterval(e.Left, ivals)
		right, okR := evalExprInterval(e.Right, ivals)
		if !okL || !okR {
			return ivl{}, false
		}
		switch e.Op {
		case token.PLUS:
			return addInterval(left, right), true
		case token.MINUS:
			return subInterval(left, right), true
		case token.STAR:
			return mulInterval(left, right), true
		}
	}
	return ivl{}, false
}

func addInterval(a, b ivl) ivl {
	return ivl{lo: satAdd(a.lo, b.lo), hi: satAdd(a.hi, b.hi), set: true}
}

func subInterval(a, b ivl) ivl {
	return ivl{lo: satSub(a.lo, b.hi), hi: satSub(a.hi, b.lo), set: true}
}

func satAdd(x, y int64) int64 {
	if y > 0 && x > math.MaxInt64-y {
		return math.MaxInt64
	}
	if y < 0 && x < math.MinInt64-y {
		return math.MinInt64
	}
	return x + y
}

func satSub(x, y int64) int64 {
	return satAdd(x, -y)
}

func mulInterval(a, b ivl) ivl {
	cands := []int64{
		satMul(a.lo, b.lo), satMul(a.lo, b.hi), satMul(a.hi, b.lo), satMul(a.hi, b.hi),
	}
	lo, hi := cands[0], cands[0]
	for _, v := range cands[1:] {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return ivl{lo: lo, hi: hi, set: true}
}

func satMul(x, y int64) int64 {
	if x == 0 || y == 0 {
		return 0
	}
	if x > 0 && y > 0 && x > math.MaxInt64/y {
		return math.MaxInt64
	}
	if x < 0 && y < 0 && x < math.MaxInt64/y {
		return math.MaxInt64
	}
	if x > 0 && y < 0 && y < math.MinInt64/x {
		return math.MinInt64
	}
	if x < 0 && y > 0 && x < math.MinInt64/y {
		return math.MinInt64
	}
	return x * y
}

// --- expression equality (shallow structural comparison) ---

func exprEqual(a, b ast.Expr) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	switch a := a.(type) {
	case *ast.IdentExpr:
		b, ok := b.(*ast.IdentExpr)
		return ok && a.Name == b.Name
	case *ast.LitExpr:
		b, ok := b.(*ast.LitExpr)
		return ok && fmtLit(a) == fmtLit(b)
	case *ast.BinaryExpr:
		b, ok := b.(*ast.BinaryExpr)
		return ok && a.Op == b.Op && exprEqual(a.Left, b.Left) && exprEqual(a.Right, b.Right)
	case *ast.ParenExpr:
		b, ok := b.(*ast.ParenExpr)
		return ok && exprEqual(a.Inner, b.Inner)
	}
	return false
}

func fmtLit(e *ast.LitExpr) string {
	return fmt.Sprintf("%v:%d", e.Value, e.Kind)
}

// --- utility ---

func int64FromValue(v any) (int64, bool) {
	switch v := v.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		if v == float64(int64(v)) {
			return int64(v), true
		}
	}
	return 0, false
}

func int64FromExpr(e ast.Expr) (int64, bool) {
	if lit, ok := e.(*ast.LitExpr); ok {
		return int64FromValue(lit.Value)
	}
	return 0, false
}
