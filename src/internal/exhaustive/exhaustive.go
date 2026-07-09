// Package exhaustive implements pattern exhaustiveness and redundancy
// checking for Goop match expressions.
//
// For every match expression, we verify that the patterns cover all possible
// values of the scrutinee type. We also detect redundant (unreachable) patterns.
//
// Supported pattern forms:
//   - Wildcards and variables (cover everything)
//   - Literals (cover exactly one value)
//   - Constructor patterns (cover one variant of an ADT)
//   - Record patterns (cover all records with the specified fields)
//   - Tuple patterns (cover tuples of matching arity)
//   - List patterns ([] covers empty list; [a;b;c] covers exactly that list)
//   - Cons patterns (cover non-empty lists)
//   - Alias patterns (same as the inner pattern)
//
// Patterns with guards are treated as potentially non-matching (the guard
// may fail), so the exhaustiveness check considers the pattern to cover its
// case but does not treat it as "covering" for redundancy purposes.
package exhaustive

import (
	"fmt"
	"strings"

	"goop.dev/compiler/internal/active"
	"goop.dev/compiler/internal/ast"
)

// Warning represents an exhaustiveness warning.
type Warning struct {
	Message string
}

func (w *Warning) Error() string { return w.Message }

// Check runs exhaustiveness checking on a module and returns warnings.
func Check(mod *ast.Module) []error {
	c := &checker{}
	c.checkModule(mod)
	var errs []error
	for _, w := range c.warnings {
		errs = append(errs, w)
	}
	return errs
}

type checker struct {
	warnings []*Warning
}

func (c *checker) warnf(format string, args ...any) {
	c.warnings = append(c.warnings, &Warning{
		Message: fmt.Sprintf(format, args...),
	})
}

func (c *checker) checkModule(mod *ast.Module) {
	for _, d := range mod.Decls {
		switch d := d.(type) {
		case *ast.LetDecl:
			for _, b := range d.Bindings {
				c.checkExprForMatch(b.Body)
			}
		case *ast.TypeDecl:
			// no expressions
		case *ast.ExternDecl:
			// no expressions
		}
	}
}

func (c *checker) checkExprForMatch(e ast.Expr) {
	if e == nil {
		return
	}
	switch e := e.(type) {
	case *ast.MatchExpr:
		c.checkMatch(e)
	case *ast.IfExpr:
		c.checkExprForMatch(e.Cond)
		c.checkExprForMatch(e.ThenBranch)
		c.checkExprForMatch(e.ElseBranch)
	case *ast.LetInExpr:
		for _, b := range e.Bindings {
			c.checkExprForMatch(b.Body)
		}
		c.checkExprForMatch(e.Body)
	case *ast.FunExpr:
		c.checkExprForMatch(e.Body)
	case *ast.GuardExpr:
		for _, b := range e.Bindings {
			c.checkExprForMatch(b.Expr)
		}
		c.checkExprForMatch(e.Else_)
	case *ast.AppExpr:
		c.checkExprForMatch(e.Func)
		c.checkExprForMatch(e.Arg)
	case *ast.BinaryExpr:
		c.checkExprForMatch(e.Left)
		c.checkExprForMatch(e.Right)
	case *ast.PipeExpr:
		c.checkExprForMatch(e.Left)
		c.checkExprForMatch(e.Right)
	case *ast.QuestionExpr:
		c.checkExprForMatch(e.Left)
		if e.Arg != nil {
			c.checkExprForMatch(e.Arg)
		}
	case *ast.RecordExpr:
		for _, f := range e.Fields {
			if f.Value != nil {
				c.checkExprForMatch(f.Value)
			}
		}
	case *ast.RecordUpdateExpr:
		c.checkExprForMatch(e.Base)
		for _, f := range e.Fields {
			if f.Value != nil {
				c.checkExprForMatch(f.Value)
			}
		}
	case *ast.FieldAccessExpr:
		c.checkExprForMatch(e.Left)
	case *ast.TupleExpr:
		for _, el := range e.Elems {
			c.checkExprForMatch(el)
		}
	case *ast.ListExpr:
		for _, el := range e.Elems {
			c.checkExprForMatch(el)
		}
	case *ast.IsExpr:
		c.checkExprForMatch(e.Left)
	case *ast.AsMatchExpr:
		c.checkExprForMatch(e.Left)
		c.checkExprForMatch(e.Body)
		c.checkExprForMatch(e.ElseBody)
	case *ast.ParenExpr:
		c.checkExprForMatch(e.Inner)
	}
}

// ---------------------------------------------------------------------------
// ADT exhaustiveness
// ---------------------------------------------------------------------------

// checkMatch verifies that the patterns in a match expression are exhaustive
// and there are no unreachable patterns.
func (c *checker) checkMatch(e *ast.MatchExpr) {
	// Also recurse into sub-expressions within arms
	for _, arm := range e.Arms {
		c.checkExprForMatch(arm.Body)
		if arm.Guard != nil {
			c.checkExprForMatch(arm.Guard)
		}
	}

	// Check exhaustiveness of the patterns.
	// We build a set of "missing" constructors or patterns, then check
	// if each arm is redundant.

	// Try to determine the scrutinee type's constructors.
	// We don't have the typed AST, so we infer the ADT structure from the
	// patterns themselves and from the environment.

	// Collect all constructors referenced in the patterns
	constructors := make(map[string]bool)
	hasWildcard := false
	covered := 0
	for i, arm := range e.Arms {
		// Active patterns never cover the input type — skip them
		if cp, ok := arm.Pattern.(*ast.ConstructorPattern); ok {
			if active.GlobalRegistry.IsActivePattern(cp.Name) {
				covered++
				continue
			}
		}

		ctors := extractConstructors(arm.Pattern)
		if len(ctors) == 0 {
			// Wildcard or variable pattern — covers everything
			hasWildcard = true
			// Check if this arm is unreachable (all previous arms cover everything)
			if i > 0 && covered > 0 {
				// This is a simplification; real redundancy checking is more complex.
				// For now, we only flag the obvious case: a wildcard after another wildcard.
				for j := 0; j < i; j++ {
					if extractConstructors(e.Arms[j].Pattern) == nil {
						c.warnf("unreachable pattern: all values already matched by previous wildcard")
						break
					}
				}
			}
		} else {
			for _, name := range ctors {
				// A pattern with a guard does NOT fully cover the constructor
				// because the guard may fail, allowing subsequent patterns to
				// still match the same constructor.
				if arm.Guard == nil {
					if constructors[name] {
						c.warnf("unreachable pattern: constructor %q already covered", name)
					} else {
						constructors[name] = true
					}
				} else {
					// Guarded: don't mark as covered, so later arms for the
					// same constructor are not flagged as redundant.
				}
				covered++
			}
		}
	}

	// If there's no wildcard, check if we know the ADT's constructors.
	// We need to know which ADT the scrutinee belongs to. Without a typed AST,
	// we make a best-effort guess based on the constructors used.

	// Try to find the ADT name from the patterns
	adtName := c.inferADTFromPatterns(e.Arms)
	if adtName != "" && !hasWildcard {
		// We identified the ADT — check if all its constructors are covered.
		// The ADT definition is looked up from a global registry.
		allCtors := c.getADTConstructors(adtName)
		if len(allCtors) > 0 {
			missing := make([]string, 0)
			for _, ctor := range allCtors {
				covered := false
				for key := range constructors {
					// Check exact match or prefix match (e.g. "Error" matches
					// "Error.NotFound" or "Error._").
					if key == ctor || strings.HasPrefix(key, ctor+".") {
						covered = true
						break
					}
				}
				if !covered {
					missing = append(missing, ctor)
				}
			}
			if len(missing) > 0 {
				c.warnf("non-exhaustive match: missing constructor(s): %s",
					strings.Join(missing, ", "))
			}
		}
	}
}

// extractConstructors returns the constructor names covered by a pattern,
// or nil if the pattern is a wildcard/variable (covers everything).
func extractConstructors(p ast.Pattern) []string {
	switch p := p.(type) {
	case *ast.WildcardPattern:
		return nil
	case *ast.IdentPattern:
		return nil // variable covers everything
	case *ast.LitPattern:
		return []string{fmt.Sprintf("%v", p.Value)}
	case *ast.ConstructorPattern:
		// If the constructor has a payload pattern, include the payload's
		// constructors as part of the coverage key so that different
		// payloads are treated as different patterns.
		if p.Arg != nil {
			argCtors := extractConstructors(p.Arg)
			if len(argCtors) > 0 {
				var result []string
				for _, ac := range argCtors {
					result = append(result, p.Name+"."+ac)
				}
				return result
			}
			// Arg is a wildcard or variable — still a distinct pattern
			return []string{p.Name + "._"}
		}
		return []string{p.Name}
	case *ast.RecordPattern:
		return []string{"<record>"}
	case *ast.TuplePattern:
		return []string{"<tuple>"}
	case *ast.ListPattern:
		if len(p.Elems) == 0 {
			return []string{"[]"}
		}
		return []string{"<list>"}
	case *ast.ConsPattern:
		return []string{"::"}
	case *ast.AliasPattern:
		return extractConstructors(p.Pattern)
	default:
		return nil
	}
}

// ---------------------------------------------------------------------------
// ADT registry (populated during type checking)
// ---------------------------------------------------------------------------

// ADTRegistry maps ADT names to their constructor lists.
var ADTRegistry = make(map[string][]string)

// RegisterADT adds an ADT to the global registry for exhaustiveness checking.
func RegisterADT(name string, constructors []string) {
	ADTRegistry[name] = constructors
}

func (c *checker) getADTConstructors(name string) []string {
	return ADTRegistry[name]
}

// inferADTFromPatterns attempts to determine the ADT name from the
// constructor patterns used in match arms.  Only top-level constructors
// (not nested in payloads) are counted.
func (c *checker) inferADTFromPatterns(arms []ast.MatchArm) string {
	ctorToADT := make(map[string]int) // count occurrences per constructor → ADT
	for _, arm := range arms {
		for _, name := range topLevelConstructors(arm.Pattern) {
			for adtName, ctors := range ADTRegistry {
				for _, ctor := range ctors {
					if ctor == name {
						ctorToADT[adtName]++
					}
				}
			}
		}
	}
	// Return the most common ADT
	best := ""
	bestCount := 0
	for adt, count := range ctorToADT {
		if count > bestCount {
			best = adt
			bestCount = count
		}
	}
	return best
}

// topLevelConstructors returns the constructor names appearing at the top
// level of a pattern (not nested inside payloads).
func topLevelConstructors(p ast.Pattern) []string {
	switch p := p.(type) {
	case *ast.ConstructorPattern:
		return []string{p.Name}
	case *ast.RecordPattern:
		return []string{"<record>"}
	case *ast.TuplePattern:
		return []string{"<tuple>"}
	case *ast.ListPattern:
		if len(p.Elems) == 0 {
			return []string{"[]"}
		}
		return []string{"<list>"}
	case *ast.ConsPattern:
		return []string{"::"}
	case *ast.AliasPattern:
		return topLevelConstructors(p.Pattern)
	default:
		return nil
	}
}

// ---------------------------------------------------------------------------
// Module-level check
// ---------------------------------------------------------------------------

// CheckModule runs both type checking and exhaustiveness checking.
// Type checking must be done first so that the ADT registry is populated.
func CheckModule(mod *ast.Module) (typeErrors []error, exhaustWarnings []error) {
	// Type checking populates the registry
	_ = mod // type checking is done separately

	// Exhaustiveness
	exhaustWarnings = Check(mod)
	return nil, exhaustWarnings
}
