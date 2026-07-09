// Package color provides ANSI terminal colorization for Goop tokens
package color

import (
	"goop.dev/compiler/internal/token"
)

// ANSI color codes
const (
	Reset   = "\x1b[0m"
	Red     = "\x1b[31m"
	Green   = "\x1b[32m"
	Yellow  = "\x1b[33m"
	Blue    = "\x1b[34m"
	Cyan    = "\x1b[36m"
	Magenta = "\x1b[35m"
	Gray    = "\x1b[90m"
)

// TokenColor maps token types to ANSI colors
func TokenColor(t token.TokenType) string {
	switch t {
	case token.LET, token.REC, token.MUTABLE, token.IF, token.THEN, token.ELSE,
		token.IN, token.AND, token.PANIC, token.WHERE, token.GUARD:
		return Cyan
	case token.TYPE, token.WITH, token.OF:
		return Green
	case token.MATCH, token.VAL, token.IS, token.REQUIRES, token.RETURNS, token.AS, token.WHEN, token.NOT:
		return Magenta
	case token.MODULE, token.OPEN, token.EXTERN, token.IMPORT:
		return Yellow
	case token.CHAN, token.GO, token.USING:
		return Red
	case token.TRUE, token.FALSE, token.UNIT:
		return Green
	case token.INT:
		return Blue
	case token.FLOAT:
		return Cyan
	case token.STRING:
		return Green
	case token.CONSTRUCTOR:
		return Yellow
	case token.IDENT:
		return Cyan
	case token.ERROR:
		return Red
	default:
		return ""
	}
}

// Colorize returns the token string wrapped in ANSI color codes
func Colorize(toks []token.Token) []string {
	lines := make([]string, len(toks))
	for i, t := range toks {
		color := TokenColor(t.Type)
		if color != "" {
			lines[i] = color + t.String() + Reset
		} else {
			lines[i] = t.String()
		}
	}
	return lines
}
