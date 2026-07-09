// Package token defines the lexical token types for the Goop language.
package token

import "fmt"

// TokenType enumerates all token kinds recognised by the lexer.
type TokenType int

const (
	EOF   TokenType = iota
	ERROR           // lexer error token
	// NEWLINE is emitted for significant line breaks (layout).
	// Currently the parser handles layout via column tracking, so the
	// lexer skips whitespace entirely.  Reserved for future use.
	_ // was NEWLINE

	// --- identifiers & literals ---
	IDENT       // lowercase or underscore-starting identifier
	TYVAR       // 'a — type variable (leading apostrophe)
	CONSTRUCTOR // Capitalised identifier (module names, ADT constructors)
	INT         // integer literal
	FLOAT       // floating-point literal
	STRING      // double-quoted string literal
	CHAR        // character literal (single-quoted)

	// --- keywords ---
	LET
	REC
	MUTABLE
	TYPE
	MATCH
	WITH
	IF
	THEN
	ELSE
	FUN
	MODULE
	OPEN
	EXTERN
	IMPORT
	GOLANG // import golang "path"
	GOOP   // import goop "path"
	AS
	WHEN
	OF
	IN
	AND
	PANIC
	TRUE
	FALSE
	UNIT
	VAL
	GUARD
	IS
	REQUIRES
	RETURNS
	WHERE
	NEWTYPE // newtype keyword for nominal wrappers

	PRIVATE // private visibility modifier

	// --- delimiters ---
	LPAREN   // (
	RPAREN   // )
	LBRACE   // {
	RBRACE   // }
	LBRACKET // [
	RBRACKET // ]

	// --- operators & symbols ---
	AT         // @  (attribute prefix, e.g. @golang { ... })
	ARROW      // ->
	PIPE       // |
	EQUALS     // =
	SEMI       // ;
	COLON      // :
	DOT        // .
	COMMA      // ,
	STAR       // *
	STARDOT    // *.  (float multiply)
	PLUS       // +
	MINUS      // -
	SLASH      // /
	UNDERSCORE // _
	CONS       // ::
	QUESTION   // ?
	PIPEOP     // |>
	DIAMOND    // <>
	NEQ        // !=
	EQEQ       // ==
	LEQ        // <=
	GEQ        // >=
	LT         // <
	GT         // >
	CARET      // ^
	LARROW     // <-
	AMPAMP     // &&
	PIPEPIPE   // ||
	NOT        // not  (logical not — not a token, 'not' is a keyword)
	BANG       // !   (used for let! and do! in computation expressions)
	PLUSDOT    // +.  (float addition)
	MINUSDOT   // -.  (float subtraction)
	SLASHDOT   // /.  (float division)

	PERCENT // %  (integer modulo)

	// --- concurrency ---
	CHAN  // chan type
	GO    // go keyword
	MOVE  // move keyword (go move list)
	USING // using keyword

	// --- imperative / OCaml surface ---
	FOR
	TO
	DO
	DONE
	BEGIN
	END

	tokenCount // internal count
)

var tokenNames = [...]string{
	EOF:         "EOF",
	ERROR:       "ERROR",
	IDENT:       "IDENT",
	TYVAR:       "TYVAR",
	CONSTRUCTOR: "CONSTRUCTOR",
	INT:         "INT",
	FLOAT:       "FLOAT",
	STRING:      "STRING",
	CHAR:        "CHAR",
	LET:         "let",
	REC:         "rec",
	MUTABLE:     "mutable",
	TYPE:        "type",
	MATCH:       "match",
	WITH:        "with",
	IF:          "if",
	THEN:        "then",
	ELSE:        "else",
	FUN:         "fun",
	MODULE:      "module",
	OPEN:        "open",
	EXTERN:      "extern",
	IMPORT:      "import",
	GOLANG:      "golang",
	GOOP:        "goop",
	AS:          "as",
	WHEN:        "when",
	OF:          "of",
	IN:          "in",
	AND:         "and",
	PANIC:       "panic",
	TRUE:        "true",
	FALSE:       "false",
	UNIT:        "()",
	VAL:         "val",
	GUARD:       "guard",
	IS:          "is",
	REQUIRES:    "requires",
	RETURNS:     "returns",
	WHERE:       "where",
	NEWTYPE:     "newtype",
	PRIVATE:     "private",
	LPAREN:      "(",
	RPAREN:      ")",
	LBRACE:      "{",
	RBRACE:      "}",
	LBRACKET:    "[",
	RBRACKET:    "]",
	AT:          "@",
	ARROW:       "->",
	PIPE:        "|",
	EQUALS:      "=",
	SEMI:        ";",
	COLON:       ":",
	DOT:         ".",
	COMMA:       ",",
	STAR:        "*",
	STARDOT:     "*.",
	PLUS:        "+",
	MINUS:       "-",
	SLASH:       "/",
	UNDERSCORE:  "_",
	CONS:        "::",
	QUESTION:    "?",
	PIPEOP:      "|>",
	DIAMOND:     "<>",
	NEQ:         "!=",
	EQEQ:        "==",
	LEQ:         "<=",
	GEQ:         ">=",
	LT:          "<",
	GT:          ">",
	CARET:       "^",
	LARROW:      "<-",
	AMPAMP:      "&&",
	PIPEPIPE:    "||",
	BANG:        "!",
	PLUSDOT:     "+.",
	MINUSDOT:    "-.",
	SLASHDOT:    "/.",
	PERCENT:     "%",
	CHAN:        "chan",
	GO:          "go",
	MOVE:        "move",
	USING:       "using",
	FOR:         "for",
	TO:          "to",
	DO:          "do",
	DONE:        "done",
	BEGIN:       "begin",
	END:         "end",
}

// String returns the human-readable name of the token type.
func (t TokenType) String() string {
	if int(t) < len(tokenNames) {
		return tokenNames[t]
	}
	return fmt.Sprintf("TokenType(%d)", int(t))
}

// IsKeyword reports whether t is a reserved word.
func (t TokenType) IsKeyword() bool {
	switch t {
	case LET, REC, MUTABLE, TYPE, MATCH, WITH, IF, THEN, ELSE, FUN,
		MODULE, OPEN, EXTERN, IMPORT, GOLANG, GOOP, AS, WHEN, OF, IN, AND, PANIC,
		TRUE, FALSE, UNIT, VAL, GUARD, IS, REQUIRES, RETURNS, WHERE, PRIVATE, CHAN, GO, MOVE, USING,
		FOR, TO, DO, DONE, BEGIN, END:
		return true
	}
	return false
}

// SourceLoc carries file/line/column/offset information.
type SourceLoc struct {
	File   string // source filename
	Line   int    // 1‑based
	Column int    // 1‑based
	Offset int    // byte offset (0‑based)
}

func (l SourceLoc) String() string {
	return fmt.Sprintf("%s:%d:%d", l.File, l.Line, l.Column)
}

// Token is a single lexical token produced by the lexer.
type Token struct {
	Type    TokenType
	Lexeme  string
	Literal any // parsed value for number/string literals (int64, float64, string)
	Loc     SourceLoc
}

func (t Token) String() string {
	switch t.Type {
	case IDENT, TYVAR, CONSTRUCTOR:
		return fmt.Sprintf("%s %q", t.Type, t.Lexeme)
	case INT:
		return fmt.Sprintf("INT %d", t.Literal)
	case FLOAT:
		return fmt.Sprintf("FLOAT %g", t.Literal)
	case STRING:
		return fmt.Sprintf("STRING %q", t.Literal)
	case CHAR:
		return fmt.Sprintf("CHAR %q", t.Literal)
	case ERROR:
		return fmt.Sprintf("ERROR %q", t.Lexeme)
	case EOF:
		return "EOF"
	default:
		return t.Type.String()
	}
}

// LookupKeyword returns the keyword token type for an identifier string,
// or IDENT if the string is not a keyword.
func LookupKeyword(s string) TokenType {
	switch s {
	case "let":
		return LET
	case "rec":
		return REC
	case "mutable":
		return MUTABLE
	case "type":
		return TYPE
	case "match":
		return MATCH
	case "with":
		return WITH
	case "if":
		return IF
	case "then":
		return THEN
	case "else":
		return ELSE
	case "fun":
		return FUN
	case "module":
		return MODULE
	case "open":
		return OPEN
	case "extern":
		return EXTERN
	case "import":
		return IMPORT
	case "golang":
		return GOLANG
	case "goop":
		return GOOP
	case "as":
		return AS
	case "when":
		return WHEN
	case "of":
		return OF
	case "in":
		return IN
	case "and":
		return AND
	case "panic":
		return PANIC
	case "true":
		return TRUE
	case "false":
		return FALSE
	case "()":
		return UNIT
	case "val":
		return VAL
	case "guard":
		return GUARD
	case "is":
		return IS
	case "requires":
		return REQUIRES
	case "returns":
		return RETURNS
	case "where":
		return WHERE
	case "newtype":
		return NEWTYPE
	case "private":
		return PRIVATE
	case "not":
		return NOT
	case "chan":
		return CHAN
	case "go":
		return GO
	case "move":
		return MOVE
	case "using":
		return USING
	case "for":
		return FOR
	case "to":
		return TO
	case "do":
		return DO
	case "done":
		return DONE
	case "begin":
		return BEGIN
	case "end":
		return END
	}
	return IDENT
}
