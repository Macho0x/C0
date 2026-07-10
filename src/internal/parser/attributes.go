package parser

import (
	"strings"

	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/token"
)

// stripAttributes removes OCaml attributes before the grammar sees them.  Goop
// deliberately has no PPX implementation: attributes are retained only as
// source metadata, while extension payloads remain as ordinary Goop syntax.
func stripAttributes(tokens []token.Token) ([]token.Token, []ast.Attribute) {
	out := make([]token.Token, 0, len(tokens))
	var attrs []ast.Attribute

	for i := 0; i < len(tokens); {
		if tokens[i].Type != token.LBRACKET || i+2 >= len(tokens) {
			out = append(out, tokens[i])
			i++
			continue
		}

		first := tokens[i+1].Type
		if first != token.AT && first != token.PERCENT {
			out = append(out, tokens[i])
			i++
			continue
		}

		end, ok := attributeEnd(tokens, i)
		if !ok {
			out = append(out, tokens[i])
			i++
			continue
		}

		j := i + 2
		attached := "expr"
		extension := first == token.PERCENT
		if first == token.AT && j < end && tokens[j].Type == token.AT {
			attached = "item"
			j++
		}
		if extension {
			attached = "ext"
			if j < end && tokens[j].Type == token.PERCENT {
				j++
			}
		}

		name := ""
		if j < end {
			name = tokens[j].Lexeme
			j++
		}
		attr := ast.Attribute{Name: name, Attached: attached}
		attr.Payload = tokenText(tokens[j:end])
		attrs = append(attrs, attr)

		if extension {
			// Extension nodes are transparent in Goop. Keep their payload so
			// [%ext 42] remains the ordinary expression 42.
			out = append(out, tokens[j:end]...)
		}
		i = end + 1
	}
	return out, attrs
}

func attributeEnd(tokens []token.Token, start int) (int, bool) {
	depth := 0
	for i := start; i < len(tokens); i++ {
		switch tokens[i].Type {
		case token.LBRACKET:
			depth++
		case token.RBRACKET:
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

func tokenText(tokens []token.Token) string {
	parts := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if tok.Lexeme != "" {
			parts = append(parts, tok.Lexeme)
		}
	}
	return strings.Join(parts, " ")
}
