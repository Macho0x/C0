package refine

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/token"
)

// CheckWithSMT tries Z3 when available, then falls back to the interval solver.
func CheckWithSMT(pred ast.Expr, constraints []ast.Expr, useSMT bool) Result {
	if useSMT {
		if r, ok := tryZ3(pred, constraints); ok {
			return r
		}
	}
	return Check(pred, constraints)
}

func tryZ3(pred ast.Expr, constraints []ast.Expr) (Result, bool) {
	if _, err := exec.LookPath("z3"); err != nil {
		return Unknown, false
	}
	smt := buildSMT2(pred, constraints)
	cmd := exec.Command("z3", "-in", "-t:2000")
	cmd.Stdin = strings.NewReader(smt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()
	select {
	case err := <-done:
		if err != nil && stdout.Len() == 0 {
			return Unknown, false
		}
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		return Unknown, false
	}
	out := strings.TrimSpace(stdout.String())
	switch {
	case strings.Contains(out, "unsat"):
		// Negated goal unsat ⇒ predicate holds
		return Proven, true
	case strings.HasPrefix(out, "sat"):
		return Disproven, true
	default:
		return Unknown, true
	}
}

func buildSMT2(pred ast.Expr, constraints []ast.Expr) string {
	vars := map[string]bool{}
	collectVars(pred, vars)
	for _, c := range constraints {
		collectVars(c, vars)
	}
	var b strings.Builder
	b.WriteString("; goop refinement VC\n")
	for v := range vars {
		fmt.Fprintf(&b, "(declare-const %s Int)\n", sanitize(v))
	}
	for _, c := range constraints {
		if s := exprToSMT(c); s != "" {
			fmt.Fprintf(&b, "(assert %s)\n", s)
		}
	}
	if s := exprToSMT(pred); s != "" {
		// Check validity: assert ¬pred; unsat ⇒ proven
		fmt.Fprintf(&b, "(assert (not %s))\n", s)
	}
	b.WriteString("(check-sat)\n")
	return b.String()
}

func sanitize(name string) string {
	r := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, name)
	if r == "" {
		return "v"
	}
	if r[0] >= '0' && r[0] <= '9' {
		return "v_" + r
	}
	return r
}

func collectVars(e ast.Expr, vars map[string]bool) {
	if e == nil {
		return
	}
	switch e := e.(type) {
	case *ast.IdentExpr:
		vars[e.Name] = true
	case *ast.BinaryExpr:
		collectVars(e.Left, vars)
		collectVars(e.Right, vars)
	case *ast.AppExpr:
		collectVars(e.Func, vars)
		collectVars(e.Arg, vars)
	case *ast.ParenExpr:
		collectVars(e.Inner, vars)
	}
}

func exprToSMT(e ast.Expr) string {
	if e == nil {
		return ""
	}
	switch e := e.(type) {
	case *ast.LitExpr:
		switch e.Kind {
		case token.INT:
			return fmt.Sprintf("%v", e.Value)
		case token.TRUE:
			return "true"
		case token.FALSE:
			return "false"
		default:
			return ""
		}
	case *ast.IdentExpr:
		return sanitize(e.Name)
	case *ast.ParenExpr:
		return exprToSMT(e.Inner)
	case *ast.BinaryExpr:
		l, r := exprToSMT(e.Left), exprToSMT(e.Right)
		if l == "" || r == "" {
			return ""
		}
		switch e.Op {
		case token.PLUS:
			return fmt.Sprintf("(+ %s %s)", l, r)
		case token.MINUS:
			return fmt.Sprintf("(- %s %s)", l, r)
		case token.STAR:
			return fmt.Sprintf("(* %s %s)", l, r)
		case token.LT:
			return fmt.Sprintf("(< %s %s)", l, r)
		case token.GT:
			return fmt.Sprintf("(> %s %s)", l, r)
		case token.LEQ:
			return fmt.Sprintf("(<= %s %s)", l, r)
		case token.GEQ:
			return fmt.Sprintf("(>= %s %s)", l, r)
		case token.EQEQ, token.EQUALS:
			return fmt.Sprintf("(= %s %s)", l, r)
		case token.NEQ, token.DIAMOND:
			return fmt.Sprintf("(not (= %s %s))", l, r)
		case token.AMPAMP:
			return fmt.Sprintf("(and %s %s)", l, r)
		case token.PIPEPIPE:
			return fmt.Sprintf("(or %s %s)", l, r)
		default:
			return ""
		}
	default:
		return ""
	}
}
