package fmt

import (
	"fmt"

	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/token"
)

// Precedence mirrors parser.go binding strengths.
const (
	precLowest  = 1
	precPipe    = 2
	precOr      = 3
	precAnd     = 4
	precCompare = 5
	precCons    = 6
	precAdd     = 7
	precMul     = 8
	precApp     = 9
	precUnary   = 10
	precPostfix = 11
)

func binPrec(op token.TokenType) int {
	switch op {
	case token.PIPEOP:
		return precPipe
	case token.PIPEPIPE:
		return precOr
	case token.AMPAMP:
		return precAnd
	case token.EQEQ, token.NEQ, token.LT, token.GT, token.LEQ, token.GEQ, token.DIAMOND:
		return precCompare
	case token.CONS:
		return precCons
	case token.PLUS, token.MINUS, token.CARET, token.LARROW, token.PLUSDOT, token.MINUSDOT:
		return precAdd
	case token.STAR, token.SLASH, token.STARDOT, token.SLASHDOT, token.PERCENT:
		return precMul
	default:
		return precLowest
	}
}

func opLexeme(op token.TokenType) string {
	return op.String()
}

func formatExprInline(e ast.Expr, minPrec int) string {
	if e == nil {
		return "()"
	}
	switch e := e.(type) {
	case *ast.LitExpr:
		return fmt.Sprintf("%v", e.Value)
	case *ast.IdentExpr:
		return e.Name
	case *ast.ConstructorExpr:
		if e.Arg != nil {
			return e.Name + " " + formatExprInline(e.Arg, precApp)
		}
		return e.Name
	case *ast.AppExpr:
		s := formatExprInline(e.Func, precApp) + " " + formatExprInline(e.Arg, precApp)
		if minPrec > precApp {
			return "(" + s + ")"
		}
		return s
	case *ast.BinaryExpr:
		p := binPrec(e.Op)
		left := formatExprInline(e.Left, p)
		right := formatExprInline(e.Right, p-1)
		if e.Op == token.CONS {
			right = formatExprInline(e.Right, p)
		}
		s := left + " " + opLexeme(e.Op) + " " + right
		if p < minPrec {
			return "(" + s + ")"
		}
		return s
	case *ast.PipeExpr:
		s := formatExprInline(e.Left, precPipe) + " |> " + formatExprInline(e.Right, precPipe)
		if precPipe < minPrec {
			return "(" + s + ")"
		}
		return s
	case *ast.QuestionExpr:
		s := formatExprInline(e.Left, precPostfix)
		if e.Arg != nil {
			s += " ? " + formatExprInline(e.Arg, precLowest)
		} else {
			s += " ?"
		}
		if precPostfix < minPrec {
			return "(" + s + ")"
		}
		return s
	case *ast.FieldAccessExpr:
		s := formatExprInline(e.Left, precPostfix) + "." + e.Field
		if precPostfix < minPrec {
			return "(" + s + ")"
		}
		return s
	case *ast.TupleExpr:
		var elems []string
		for _, el := range e.Elems {
			elems = append(elems, formatExprInline(el, precLowest))
		}
		return "(" + stringsJoin(elems, ", ") + ")"
	case *ast.ListExpr:
		var elems []string
		for _, el := range e.Elems {
			elems = append(elems, formatExprInline(el, precLowest))
		}
		return "[" + stringsJoin(elems, "; ") + "]"
	case *ast.RecordExpr:
		var fields []string
		for _, f := range e.Fields {
			if f.Value != nil {
				fields = append(fields, f.Name+" = "+formatExprInline(f.Value, precLowest))
			} else {
				fields = append(fields, f.Name)
			}
		}
		return "{" + stringsJoin(fields, "; ") + "}"
	case *ast.RecordUpdateExpr:
		var fields []string
		for _, f := range e.Fields {
			fields = append(fields, f.Name+" = "+formatExprInline(f.Value, precLowest))
		}
		return "{" + formatExprInline(e.Base, precLowest) + " with " + stringsJoin(fields, "; ") + "}"
	case *ast.ParenExpr:
		return "(" + formatExprInline(e.Inner, precLowest) + ")"
	case *ast.FunExpr:
		var ps []string
		for _, p := range e.Params {
			s := p.Name
			if p.Type != nil {
				s += " : " + formatType(p.Type)
			}
			ps = append(ps, s)
		}
		return "fun " + stringsJoin(ps, " ") + " -> " + formatExprInline(e.Body, precLowest)
	case *ast.IfExpr:
		return formatIfInline(e)
	case *ast.MatchExpr:
		return formatMatchInline(e)
	case *ast.LetInExpr:
		return formatLetInInline(e)
	case *ast.GuardExpr:
		return formatGuardInline(e)
	case *ast.IsExpr:
		return formatExprInline(e.Left, precCompare) + " is " + formatPattern(e.Pattern)
	case *ast.AsMatchExpr:
		return formatExprInline(e.Left, precCompare) + " as " + formatPattern(e.Pattern) +
			" -> " + formatExprInline(e.Body, precLowest) + " else " + formatExprInline(e.ElseBody, precLowest)
	case *ast.CompExpr:
		return formatCompRegion(e.Builder, e.Ops)
	case *ast.RegionExpr:
		return formatCompRegion("region", e.Ops)
	case *ast.GoExpr:
		if len(e.Moved) > 0 {
			return "go (move " + stringsJoin(e.Moved, ", ") + ") " + formatExprInline(e.Expr, precLowest)
		}
		return "go " + formatExprInline(e.Expr, precLowest)
	case *ast.SelectExpr:
		return formatSelectInline(e)
	case *ast.UsingExpr:
		return "using " + formatPattern(e.Pattern) + " = " + formatExprInline(e.Expr, precLowest) +
			" in " + formatExprInline(e.Body, precLowest)
	default:
		return "<expr>"
	}
}

func formatIfInline(e *ast.IfExpr) string {
	s := "if " + formatExprInline(e.Cond, precLowest) + " then " + formatExprInline(e.ThenBranch, precLowest)
	if elseIf, ok := e.ElseBranch.(*ast.IfExpr); ok {
		s += " else " + formatIfInline(elseIf)
	} else {
		s += " else " + formatExprInline(e.ElseBranch, precLowest)
	}
	return s
}

func formatMatchInline(e *ast.MatchExpr) string {
	var arms []string
	for _, a := range e.Arms {
		arm := formatPattern(a.Pattern)
		if a.Guard != nil {
			arm += " when " + formatExprInline(a.Guard, precLowest)
		}
		arm += " -> " + formatExprInline(a.Body, precLowest)
		arms = append(arms, arm)
	}
	return "match " + formatExprInline(e.Scrutinee, precLowest) + " with " + stringsJoin(arms, " | ")
}

func formatLetInInline(e *ast.LetInExpr) string {
	var bs []string
	prefix := "let "
	if e.Mutable {
		prefix = "let mutable "
	}
	for _, b := range e.Bindings {
		bs = append(bs, prefix+b.Name+" = "+formatExprInline(b.Body, precLowest))
		prefix = "and "
	}
	s := stringsJoin(bs, " ")
	if e.Body != nil {
		s += " in " + formatExprInline(e.Body, precLowest)
	}
	return s
}

func formatGuardInline(e *ast.GuardExpr) string {
	var bs []string
	for _, b := range e.Bindings {
		bs = append(bs, formatPattern(b.Pattern)+" = "+formatExprInline(b.Expr, precLowest))
	}
	return "guard " + stringsJoin(bs, " and ") + " else " + formatExprInline(e.Else_, precLowest)
}

func formatCompRegion(builder string, ops []ast.CompOp) string {
	var parts []string
	for _, op := range ops {
		switch o := op.(type) {
		case *ast.LetBangOp:
			parts = append(parts, "let! "+formatPattern(o.Pattern)+" = "+formatExprInline(o.Expr, precLowest))
		case *ast.DoBangOp:
			parts = append(parts, "do! "+formatExprInline(o.Expr, precLowest))
		case *ast.LetOp:
			parts = append(parts, "let "+formatPattern(o.Pattern)+" = "+formatExprInline(o.Expr, precLowest))
		case *ast.ReturnOp:
			parts = append(parts, "return "+formatExprInline(o.Expr, precLowest))
		case *ast.ReturnBangOp:
			parts = append(parts, "return! "+formatExprInline(o.Expr, precLowest))
		case *ast.BodyOp:
			parts = append(parts, formatExprInline(o.Expr, precLowest))
		}
	}
	return builder + " { " + stringsJoin(parts, "; ") + " }"
}

func formatSelectInline(e *ast.SelectExpr) string {
	var cases []string
	for _, c := range e.Cases {
		cases = append(cases, "case "+c.Bind+" = "+formatExprInline(c.Recv, precLowest)+
			" -> "+formatExprInline(c.Body, precLowest))
	}
	s := "select { " + stringsJoin(cases, "; ")
	if e.Default != nil {
		s += "; default -> " + formatExprInline(e.Default, precLowest)
	}
	return s + " }"
}

func formatExprBlock(p *Printer, e ast.Expr) {
	switch e := e.(type) {
	case *ast.IfExpr:
		formatIfBlock(p, e)
	case *ast.MatchExpr:
		formatMatchBlock(p, e)
	case *ast.LetInExpr:
		formatLetInBlock(p, e)
	default:
		p.WriteIndent()
		p.Write(formatExprInline(e, precLowest))
		p.Newline()
	}
}

func formatIfBlock(p *Printer, e *ast.IfExpr) {
	p.WriteIndent()
	p.Write("if " + formatExprInline(e.Cond, precLowest) + " then")
	p.Newline()
	p.Indent()
	formatExprBlock(p, e.ThenBranch)
	p.Dedent()
	if elseIf, ok := e.ElseBranch.(*ast.IfExpr); ok {
		p.WriteIndent()
		p.Write("else")
		p.Newline()
		formatIfBlock(p, elseIf)
	} else {
		p.WriteIndent()
		p.Write("else")
		p.Newline()
		p.Indent()
		formatExprBlock(p, e.ElseBranch)
		p.Dedent()
	}
}

func formatMatchBlock(p *Printer, e *ast.MatchExpr) {
	p.WriteIndent()
	p.Write("match " + formatExprInline(e.Scrutinee, precLowest) + " with")
	p.Newline()
	for _, a := range e.Arms {
		p.WriteIndent()
		p.Write("| " + formatPattern(a.Pattern))
		if a.Guard != nil {
			p.Write(" when " + formatExprInline(a.Guard, precLowest))
		}
		p.Write(" ->")
		p.Newline()
		p.Indent()
		formatExprBlock(p, a.Body)
		p.Dedent()
	}
}

func formatLetInBlock(p *Printer, e *ast.LetInExpr) {
	prefix := "let "
	if e.Mutable {
		prefix = "let mutable "
	}
	for i, b := range e.Bindings {
		p.WriteIndent()
		if i > 0 {
			prefix = "and "
		}
		p.Write(prefix + b.Name + " = " + formatExprInline(b.Body, precLowest))
		p.Newline()
	}
	if e.Body != nil {
		p.WriteIndent()
		p.Write("in")
		p.Newline()
		p.Indent()
		formatExprBlock(p, e.Body)
		p.Dedent()
	}
}
