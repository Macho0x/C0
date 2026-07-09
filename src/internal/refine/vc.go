package refine

import (
	"fmt"

	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/token"
	"goop.dev/compiler/internal/typeinfo"
)

// CheckRefinements walks a module's function bodies and checks refinement
// contracts at each call site. Returns:
//   - provenSites: call-site AST nodes whose refinements have been proven.
//   - warnings: unproven refinements (runtime check will be emitted).
//   - errors: disproven refinements (compile error).
func CheckRefinements(mod *ast.Module, tm typeinfo.TypeMap) (ProvenSites, []error, []error) {
	provenSites := make(ProvenSites)
	var warnings []error
	var errs []error

	// First pass: collect all function definitions and their refinement-annotated
	// parameters, keyed by function name.
	funcInfo := make(map[string]*funcDef)
	for _, d := range mod.Decls {
		ld, ok := d.(*ast.LetDecl)
		if !ok {
			continue
		}
		for i := range ld.Bindings {
			b := &ld.Bindings[i]
			fd := &funcDef{
				name:       b.Name,
				params:     b.Params,
				refinements: make(map[int]*ast.RefinementType),
			}
			hasRefinement := false
			for j, p := range b.Params {
				if rt, ok := p.Type.(*ast.RefinementType); ok {
					fd.refinements[j] = rt
					hasRefinement = true
				}
			}
			if hasRefinement {
				funcInfo[b.Name] = fd
			}
		}
	}

	// Second pass: walk each function body, collecting call sites.
	for _, d := range mod.Decls {
		ld, ok := d.(*ast.LetDecl)
		if !ok {
			continue
		}
		for i := range ld.Bindings {
			b := &ld.Bindings[i]
			// Skip functions without bodies (e.g., extern)
			if b.Body == nil {
				continue
			}
			pctx := &pathCtx{
				constraints: nil,
				funcInfo:    funcInfo,
				proven:      provenSites,
				tm:          tm,
			}
			walkExpr(b.Body, pctx, &warnings, &errs)
		}
	}

	return provenSites, warnings, errs
}

// funcDef stores information about a function with refinement-annotated params.
type funcDef struct {
	name        string
	params      []ast.Param
	refinements map[int]*ast.RefinementType // param index → refinement type
}

// pathCtx tracks the current path condition during AST traversal.
type pathCtx struct {
	constraints []ast.Expr
	funcInfo    map[string]*funcDef
	proven      ProvenSites
	tm          typeinfo.TypeMap
}

// walkExpr recursively walks an expression, tracking path conditions.
func walkExpr(e ast.Expr, ctx *pathCtx, warnings *[]error, errs *[]error) {
	if e == nil {
		return
	}
	switch e := e.(type) {
	case *ast.IfExpr:
		// If: add cond to path for then-branch, not cond for else-branch
		walkExpr(e.Cond, ctx, warnings, errs)

		thenCtx := ctx.withConstraint(e.Cond)
		walkExpr(e.ThenBranch, thenCtx, warnings, errs)

		elseCtx := ctx.withConstraint(negateExpr(e.Cond))
		walkExpr(e.ElseBranch, elseCtx, warnings, errs)

	case *ast.MatchExpr:
		walkExpr(e.Scrutinee, ctx, warnings, errs)
		for _, arm := range e.Arms {
			armCtx := ctx.withConstraints(patternConstraints(arm.Pattern, e.Scrutinee))
			if arm.Guard != nil {
				armCtx = armCtx.withConstraint(arm.Guard)
			}
			walkExpr(arm.Body, armCtx, warnings, errs)
		}

	case *ast.LetInExpr:
		for i := range e.Bindings {
			if e.Bindings[i].Body != nil {
				walkExpr(e.Bindings[i].Body, ctx, warnings, errs)
			}
		}
		walkExpr(e.Body, ctx, warnings, errs)

	case *ast.AppExpr:
		// This is a call site. Extract function name and arguments from
		// the potentially-curried call chain.
		allArgs := collectArgs(e)
		calledFunc := allArgs[0].(ast.Expr)
		args := allArgs[1:]
		funcName := getFuncName(calledFunc)
		fd := ctx.funcInfo[funcName]
		if fd == nil {
			// Not a function with refinements — walk children and continue
			walkExpr(e.Func, ctx, warnings, errs)
			walkExpr(e.Arg, ctx, warnings, errs)
			return
		}

		// Walk sub-expressions
		walkExpr(calledFunc, ctx, warnings, errs)
		for _, a := range args {
			walkExpr(a.(ast.Expr), ctx, warnings, errs)
		}

		// Check each refinement param against the actual argument
		allProven := true
		for paramIdx, rt := range fd.refinements {
			if paramIdx >= len(args) {
				continue // partial application, skip
			}
			actualArg := args[paramIdx].(ast.Expr)
			paramName := fd.params[paramIdx].Name
			pred := substituteIdent(rt.Pred, paramName, actualArg)

			result := Check(pred, ctx.constraints)
			switch result {
			case Disproven:
				*errs = append(*errs, fmt.Errorf("refinement violated: cannot satisfy %s at %s",
					ast.ExprString(rt.Pred), locString(e)))
				allProven = false
			case Unknown:
				*warnings = append(*warnings, fmt.Errorf("could not prove refinement %s at %s — runtime check emitted",
					ast.ExprString(rt.Pred), locString(e)))
				allProven = false
			case Proven:
				// proven — no error, no warning
			}
		}

		if allProven && len(fd.refinements) > 0 {
			ctx.proven[e] = true
		}

	case *ast.BinaryExpr:
		walkExpr(e.Left, ctx, warnings, errs)
		walkExpr(e.Right, ctx, warnings, errs)

	case *ast.FunExpr:
		walkExpr(e.Body, ctx, warnings, errs)

	case *ast.PipeExpr:
		walkExpr(e.Left, ctx, warnings, errs)
		walkExpr(e.Right, ctx, warnings, errs)

	case *ast.GuardExpr:
		for _, b := range e.Bindings {
			walkExpr(b.Expr, ctx, warnings, errs)
		}
		walkExpr(e.Else_, ctx, warnings, errs)

	case *ast.GoExpr:
		walkExpr(e.Expr, ctx, warnings, errs)

	case *ast.TupleExpr:
		for _, el := range e.Elems {
			walkExpr(el, ctx, warnings, errs)
		}

	case *ast.ListExpr:
		for _, el := range e.Elems {
			walkExpr(el, ctx, warnings, errs)
		}

	case *ast.ParenExpr:
		walkExpr(e.Inner, ctx, warnings, errs)

	case *ast.RecordExpr:
		for _, f := range e.Fields {
			if f.Value != nil {
				walkExpr(f.Value, ctx, warnings, errs)
			}
		}

	case *ast.RecordUpdateExpr:
		walkExpr(e.Base, ctx, warnings, errs)
		for _, f := range e.Fields {
			if f.Value != nil {
				walkExpr(f.Value, ctx, warnings, errs)
			}
		}

	case *ast.FieldAccessExpr:
		walkExpr(e.Left, ctx, warnings, errs)

	case *ast.ConstructorExpr:
		if e.Arg != nil {
			walkExpr(e.Arg, ctx, warnings, errs)
		}

	case *ast.QuestionExpr:
		walkExpr(e.Left, ctx, warnings, errs)
		if e.Arg != nil {
			walkExpr(e.Arg, ctx, warnings, errs)
		}

	case *ast.CompExpr:
		for _, op := range e.Ops {
			switch o := op.(type) {
			case *ast.LetBangOp:
				walkExpr(o.Expr, ctx, warnings, errs)
			case *ast.DoBangOp:
				walkExpr(o.Expr, ctx, warnings, errs)
			case *ast.LetOp:
				walkExpr(o.Expr, ctx, warnings, errs)
			case *ast.ReturnOp:
				walkExpr(o.Expr, ctx, warnings, errs)
			case *ast.ReturnBangOp:
				walkExpr(o.Expr, ctx, warnings, errs)
			case *ast.BodyOp:
				walkExpr(o.Expr, ctx, warnings, errs)
			}
		}

	case *ast.RegionExpr:
		for _, op := range e.Ops {
			switch o := op.(type) {
			case *ast.LetBangOp:
				walkExpr(o.Expr, ctx, warnings, errs)
			case *ast.DoBangOp:
				walkExpr(o.Expr, ctx, warnings, errs)
			case *ast.LetOp:
				walkExpr(o.Expr, ctx, warnings, errs)
			case *ast.ReturnOp:
				walkExpr(o.Expr, ctx, warnings, errs)
			case *ast.ReturnBangOp:
				walkExpr(o.Expr, ctx, warnings, errs)
			case *ast.BodyOp:
				walkExpr(o.Expr, ctx, warnings, errs)
			}
		}

	case *ast.SelectExpr:
		for _, c := range e.Cases {
			walkExpr(c.Recv, ctx, warnings, errs)
			walkExpr(c.Body, ctx, warnings, errs)
		}
		if e.Default != nil {
			walkExpr(e.Default, ctx, warnings, errs)
		}

	case *ast.UsingExpr:
		walkExpr(e.Expr, ctx, warnings, errs)
		walkExpr(e.Body, ctx, warnings, errs)

	case *ast.IsExpr:
		walkExpr(e.Left, ctx, warnings, errs)

	case *ast.AsMatchExpr:
		walkExpr(e.Left, ctx, warnings, errs)
		walkExpr(e.Body, ctx, warnings, errs)
		walkExpr(e.ElseBody, ctx, warnings, errs)

	// Leaf nodes — nothing to recurse into
	case *ast.LitExpr, *ast.IdentExpr:
		// no children
	}
}

// withConstraint returns a copy of ctx with an additional constraint.
func (ctx *pathCtx) withConstraint(c ast.Expr) *pathCtx {
	newConstraints := make([]ast.Expr, len(ctx.constraints)+1)
	copy(newConstraints, ctx.constraints)
	newConstraints[len(ctx.constraints)] = c
	return &pathCtx{
		constraints: newConstraints,
		funcInfo:    ctx.funcInfo,
		proven:      ctx.proven,
		tm:          ctx.tm,
	}
}

// withConstraints returns a copy of ctx with additional constraints.
func (ctx *pathCtx) withConstraints(cs []ast.Expr) *pathCtx {
	if len(cs) == 0 {
		return ctx
	}
	newConstraints := make([]ast.Expr, len(ctx.constraints)+len(cs))
	copy(newConstraints, ctx.constraints)
	copy(newConstraints[len(ctx.constraints):], cs)
	return &pathCtx{
		constraints: newConstraints,
		funcInfo:    ctx.funcInfo,
		proven:      ctx.proven,
		tm:          ctx.tm,
	}
}

// getFuncName extracts the function name from an expression.
func getFuncName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.IdentExpr:
		return e.Name
	case *ast.FieldAccessExpr:
		// Qualified name like Console.print_line
		return e.Field
	}
	return ""
}

// collectArgs flattens a curried application chain into a slice.
// AppExpr(Fun=AppExpr(Fun=f, Arg=a), Arg=b) → [f, a, b]
func collectArgs(app *ast.AppExpr) []ast.Expr {
	var result []ast.Expr
	current := app
	for {
		result = append([]ast.Expr{current.Arg}, result...)
		if inner, ok := current.Func.(*ast.AppExpr); ok {
			current = inner
		} else {
			result = append([]ast.Expr{current.Func}, result...)
			break
		}
	}
	return result
}

// substituteIdent replaces all occurrences of an identifier with an expression.
func substituteIdent(e ast.Expr, name string, replacement ast.Expr) ast.Expr {
	if e == nil {
		return nil
	}
	switch e := e.(type) {
	case *ast.IdentExpr:
		if e.Name == name {
			return cloneExpr(replacement)
		}
		return e

	case *ast.BinaryExpr:
		return &ast.BinaryExpr{
			Left:  substituteIdent(e.Left, name, replacement),
			Op:    e.Op,
			Right: substituteIdent(e.Right, name, replacement),
		}

	case *ast.ParenExpr:
		return &ast.ParenExpr{
			Inner: substituteIdent(e.Inner, name, replacement),
		}

	case *ast.AppExpr:
		return &ast.AppExpr{
			Func: substituteIdent(e.Func, name, replacement),
			Arg:  substituteIdent(e.Arg, name, replacement),
		}

	case *ast.FieldAccessExpr:
		return &ast.FieldAccessExpr{
			Left:  substituteIdent(e.Left, name, replacement),
			Field: e.Field,
		}
	}
	return e
}

// cloneExpr creates a shallow copy of an expression for substitution.
func cloneExpr(e ast.Expr) ast.Expr {
	if e == nil {
		return nil
	}
	// Since we're only dealing with simple expressions in predicates
	// (identifiers, literals, binary ops), a shallow copy is sufficient
	// because we never modify the original.
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

// negateExpr creates a negated version of a boolean expression.
// For now handles simple cases; Unknown/unhandled returns nil (which means "don't add a constraint").
func negateExpr(e ast.Expr) ast.Expr {
	if e == nil {
		return nil
	}
	switch e := e.(type) {
	case *ast.BinaryExpr:
		switch e.Op {
		case token.EQEQ, token.EQUALS:
			return &ast.BinaryExpr{Left: e.Left, Op: token.DIAMOND, Right: e.Right}
		case token.DIAMOND:
			return &ast.BinaryExpr{Left: e.Left, Op: token.EQEQ, Right: e.Right}
		case token.NEQ:
			return &ast.BinaryExpr{Left: e.Left, Op: token.EQEQ, Right: e.Right}
		case token.LT:
			return &ast.BinaryExpr{Left: e.Left, Op: token.GEQ, Right: e.Right}
		case token.GT:
			return &ast.BinaryExpr{Left: e.Left, Op: token.LEQ, Right: e.Right}
		case token.LEQ:
			return &ast.BinaryExpr{Left: e.Left, Op: token.GT, Right: e.Right}
		case token.GEQ:
			return &ast.BinaryExpr{Left: e.Left, Op: token.LT, Right: e.Right}
		}
	case *ast.IdentExpr:
		// not x → x == false? For simplicity, "not x" → x == 0
		return &ast.BinaryExpr{Left: e, Op: token.EQEQ, Right: &ast.LitExpr{Value: int64(0), Kind: token.INT}}
	case *ast.LitExpr:
		if b, ok := e.Value.(bool); ok {
			if b {
				return &ast.LitExpr{Value: false, Kind: token.FALSE}
			}
			return &ast.LitExpr{Value: true, Kind: token.TRUE}
		}
	}
	return nil
}

// patternConstraints extracts path constraints from match patterns.
// For example: `head :: _` pattern adds the constraint `length scrutinee > 0`.
// For now, we extract simple constraints.
func patternConstraints(p ast.Pattern, scrutinee ast.Expr) []ast.Expr {
	switch p := p.(type) {
	case *ast.ConsPattern:
		// cons pattern: the scrutinee is non-empty → it's not equal to []
		// This is informational; for the solver it means the list is non-empty.
		return nil // Skip for now — lists aren't in scope for the integer solver
	case *ast.ConstructorPattern:
		// For constructors with arguments, the scrutinee has that variant
		return nil
	case *ast.LitPattern:
		// Literal pattern: scrutinee == value
		return []ast.Expr{
			&ast.BinaryExpr{
				Left:  scrutinee,
				Op:    token.EQUALS,
				Right: &ast.LitExpr{Value: p.Value, Kind: p.Kind},
			},
		}
	case *ast.IdentPattern:
		// Variable pattern: no constraint, just binding
		return nil
	}
	return nil
}

// locString returns a human-readable location string from an AST node.
func locString(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.AppExpr:
		return fmt.Sprintf("line %d", e.Loc.Line)
	case *ast.BinaryExpr:
		return fmt.Sprintf("line %d", e.Loc.Line)
	case *ast.IfExpr:
		return fmt.Sprintf("line %d", e.Loc.Line)
	case *ast.IdentExpr:
		return fmt.Sprintf("line %d", e.Loc.Line)
	case *ast.LitExpr:
		return fmt.Sprintf("line %d", e.Loc.Line)
	}
	return "<unknown location>"
}
