// Package exhaustive implements pattern exhaustiveness and redundancy
// checking for Goop match expressions.
package exhaustive

import (
	"fmt"
	"strings"

	"goop.dev/compiler/internal/active"
	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/config"
	"goop.dev/compiler/internal/token"
)

// Error is a structured exhaustiveness diagnostic.
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

// Check runs exhaustiveness checking with default config (missing = error).
func Check(mod *ast.Module) []error {
	errs, warns := CheckWithConfig(mod, config.DefaultConfig())
	out := append(errs, warns...)
	return out
}

// CheckWithConfig returns fatal errors and warnings separately per config.
func CheckWithConfig(mod *ast.Module, cfg *config.Config) (errors []error, warnings []error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	c := &checker{cfg: cfg}
	c.checkModule(mod)
	for _, e := range c.diagnostics {
		if e.Code == "EXHAUST003" && cfg.Check.ExhaustMissing == config.SeverityError {
			errors = append(errors, e)
		} else if e.Code == "EXHAUST003" && cfg.Check.ExhaustMissing == config.SeverityWarn {
			warnings = append(warnings, e)
		} else if (e.Code == "EXHAUST001" || e.Code == "EXHAUST002") && cfg.Check.ExhaustRedundant == config.SeverityError {
			errors = append(errors, e)
		} else if (e.Code == "EXHAUST001" || e.Code == "EXHAUST002") && cfg.Check.ExhaustRedundant == config.SeverityWarn {
			warnings = append(warnings, e)
		} else if cfg.Check.ExhaustRedundant != config.SeverityOff || e.Code == "EXHAUST003" {
			// EXHAUST003 with off still skipped above; redundant off skips 001/002
			if e.Code != "EXHAUST001" && e.Code != "EXHAUST002" {
				errors = append(errors, e)
			}
		}
	}
	return errors, warnings
}

type checker struct {
	cfg          *config.Config
	diagnostics  []*Error
}

func (c *checker) emit(code, msg string, loc token.SourceLoc) {
	c.diagnostics = append(c.diagnostics, &Error{Code: code, Msg: msg, Loc: loc})
}

func (c *checker) checkModule(mod *ast.Module) {
	for _, d := range mod.Decls {
		switch d := d.(type) {
		case *ast.LetDecl:
			for _, b := range d.Bindings {
				c.checkExprForMatch(b.Body)
			}
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
	case *ast.CompExpr:
		for _, op := range e.Ops {
			switch o := op.(type) {
			case *ast.LetBangOp:
				c.checkExprForMatch(o.Expr)
			case *ast.LetOp:
				c.checkExprForMatch(o.Expr)
			case *ast.DoBangOp:
				c.checkExprForMatch(o.Expr)
			case *ast.ReturnOp:
				c.checkExprForMatch(o.Expr)
			case *ast.ReturnBangOp:
				c.checkExprForMatch(o.Expr)
			case *ast.BodyOp:
				c.checkExprForMatch(o.Expr)
			}
		}
	case *ast.RegionExpr:
		for _, op := range e.Ops {
			switch o := op.(type) {
			case *ast.LetBangOp:
				c.checkExprForMatch(o.Expr)
			case *ast.LetOp:
				c.checkExprForMatch(o.Expr)
			case *ast.DoBangOp:
				c.checkExprForMatch(o.Expr)
			case *ast.ReturnOp:
				c.checkExprForMatch(o.Expr)
			case *ast.ReturnBangOp:
				c.checkExprForMatch(o.Expr)
			case *ast.BodyOp:
				c.checkExprForMatch(o.Expr)
			}
		}
	case *ast.GoExpr:
		c.checkExprForMatch(e.Expr)
	case *ast.SelectExpr:
		for _, cs := range e.Cases {
			c.checkExprForMatch(cs.Recv)
			c.checkExprForMatch(cs.Body)
		}
		if e.Default != nil {
			c.checkExprForMatch(e.Default)
		}
	}
}

func (c *checker) checkMatch(e *ast.MatchExpr) {
	for _, arm := range e.Arms {
		c.checkExprForMatch(arm.Body)
		if arm.Guard != nil {
			c.checkExprForMatch(arm.Guard)
		}
	}

	constructors := make(map[string]bool)
	hasWildcard := false
	for i, arm := range e.Arms {
		if cp, ok := arm.Pattern.(*ast.ConstructorPattern); ok {
			if active.GlobalRegistry.IsActivePattern(cp.Name) {
				continue
			}
		}

		ctors := extractConstructors(arm.Pattern)
		if len(ctors) == 0 {
			hasWildcard = true
			if i > 0 {
				for j := 0; j < i; j++ {
					if extractConstructors(e.Arms[j].Pattern) == nil {
						c.emit("EXHAUST001", "unreachable pattern: all values already matched by previous wildcard", e.Loc)
						break
					}
				}
			}
		} else {
			for _, name := range ctors {
				if arm.Guard == nil {
					if constructors[name] {
						c.emit("EXHAUST002", fmt.Sprintf("unreachable pattern: constructor %q already covered", name), e.Loc)
					} else {
						constructors[name] = true
					}
				}
			}
		}
	}

	adtName := c.inferADTFromPatterns(e.Arms)
	if adtName != "" && !hasWildcard {
		allCtors := c.getADTConstructors(adtName)
		if len(allCtors) > 0 {
			var missing []string
			for _, ctor := range allCtors {
				covered := false
				for key := range constructors {
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
				c.emit("EXHAUST003",
					fmt.Sprintf("non-exhaustive match: missing constructor(s): %s", strings.Join(missing, ", ")),
					e.Loc)
			}
		}
	}
}

func extractConstructors(p ast.Pattern) []string {
	switch p := p.(type) {
	case *ast.WildcardPattern:
		return nil
	case *ast.IdentPattern:
		return nil
	case *ast.LitPattern:
		return []string{fmt.Sprintf("%v", p.Value)}
	case *ast.ConstructorPattern:
		if p.Arg != nil {
			argCtors := extractConstructors(p.Arg)
			if len(argCtors) > 0 {
				var result []string
				for _, ac := range argCtors {
					result = append(result, p.Name+"."+ac)
				}
				return result
			}
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

var ADTRegistry = make(map[string][]string)

func RegisterADT(name string, constructors []string) {
	ADTRegistry[name] = constructors
}

func (c *checker) getADTConstructors(name string) []string {
	return ADTRegistry[name]
}

func (c *checker) inferADTFromPatterns(arms []ast.MatchArm) string {
	ctorToADT := make(map[string]int)
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

func topLevelConstructors(p ast.Pattern) []string {
	switch p := p.(type) {
	case *ast.ConstructorPattern:
		return []string{p.Name}
	case *ast.AliasPattern:
		return topLevelConstructors(p.Pattern)
	default:
		return nil
	}
}

// CheckModule is kept for compatibility.
func CheckModule(mod *ast.Module) (typeErrors []error, exhaustWarnings []error) {
	errs, warns := CheckWithConfig(mod, config.DefaultConfig())
	return nil, append(errs, warns...)
}
