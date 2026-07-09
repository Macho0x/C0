package fmt

import (
	"fmt"

	"goop.dev/compiler/internal/ast"
)

func formatPattern(p ast.Pattern) string {
	if p == nil {
		return "_"
	}
	switch p := p.(type) {
	case *ast.WildcardPattern:
		return "_"
	case *ast.IdentPattern:
		return p.Name
	case *ast.LitPattern:
		return fmt.Sprintf("%v", p.Value)
	case *ast.ConstructorPattern:
		if p.Arg != nil {
			return p.Name + " " + formatPattern(p.Arg)
		}
		return p.Name
	case *ast.RecordPattern:
		var fields []string
		for _, f := range p.Fields {
			if f.Pattern != nil {
				fields = append(fields, f.Name+" = "+formatPattern(f.Pattern))
			} else {
				fields = append(fields, f.Name)
			}
		}
		return "{" + stringsJoin(fields, "; ") + "}"
	case *ast.TuplePattern:
		var elems []string
		for _, el := range p.Elems {
			elems = append(elems, formatPattern(el))
		}
		return "(" + stringsJoin(elems, ", ") + ")"
	case *ast.ListPattern:
		if len(p.Elems) == 0 {
			return "[]"
		}
		var elems []string
		for _, el := range p.Elems {
			elems = append(elems, formatPattern(el))
		}
		return "[" + stringsJoin(elems, "; ") + "]"
	case *ast.ConsPattern:
		return formatPattern(p.Head) + " :: " + formatPattern(p.Tail)
	case *ast.AliasPattern:
		return formatPattern(p.Pattern) + " as " + p.Name
	default:
		return "_"
	}
}

func stringsJoin(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for i := 1; i < len(ss); i++ {
		out += sep + ss[i]
	}
	return out
}
