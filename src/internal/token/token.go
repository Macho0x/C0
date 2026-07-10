// Package token defines the lexical token types for the Goop language (1.0 OCaml-aligned).
package token

import "fmt"

// TokenType enumerates all token kinds recognised by the lexer.
type TokenType int

const (
	EOF   TokenType = iota
	ERROR           // lexer error token
	_               // reserved (was NEWLINE)

	// --- identifiers & literals ---
	IDENT
	TYVAR
	CONSTRUCTOR
	POLYVAR // `Tag
	INT
	FLOAT
	STRING
	CHAR

	// --- keywords ---
	LET
	REC
	MUTABLE // record fields only (not let mutable)
	TYPE
	MATCH
	WITH
	IF
	THEN
	ELSE
	FUN
	FUNCTION
	MODULE
	OPEN
	EXTERN
	IMPORT
	GOLANG
	GOOP
	AS
	WHEN
	OF
	IN
	AND
	TRUE
	FALSE
	UNIT
	VAL
	REQUIRES
	RETURNS
	WHERE
	PRIVATE
	WHILE
	EXCEPTION
	RAISE
	TRY
	FINALLY
	EFFECT
	PERFORM
	CLASS
	OBJECT
	METHOD
	INHERIT
	INITIALIZER
	VIRTUAL
	CONSTRAINT
	STRUCT
	SIG
	INCLUDE
	LAZY
	ASSERT
	FAILWITH
	MOD
	LAND
	LOR
	LXOR
	NEW
	REF // ref keyword (also type constructor)

	// Migration-only keywords (parse as errors with PARSE-MIG*)
	GUARD
	IS
	PANIC
	NEWTYPE
	USING // legacy; region CE removed

	// --- delimiters ---
	LPAREN
	RPAREN
	LBRACE
	RBRACE
	LBRACKET
	RBRACKET
	LBRACKETPIPE // [|
	PIPERBRACKET // |]

	// --- operators & symbols ---
	AT
	ARROW  // ->
	PIPE   // |
	EQUALS // =
	SEMI
	COLON
	COLONEQ // :=
	DOT
	COMMA
	STAR
	STARDOT
	PLUS
	MINUS
	SLASH
	UNDERSCORE
	CONS
	QUESTION // kept for optional-arg ?x; bare ? propagation is PARSE-MIG
	PIPEOP
	DIAMOND
	NEQ
	EQEQ
	LEQ
	GEQ
	LT
	GT
	CARET
	LARROW // <- array / mutable field only
	AMPAMP
	PIPEPIPE
	NOT
	BANG // ! deref (and legacy let! rejected)
	PLUSDOT
	MINUSDOT
	SLASHDOT
	PERCENT // migration: use mod
	BACKTICK
	TILDE // ~ labelled arguments

	// --- concurrency ---
	CHAN
	GO
	MOVE

	// --- imperative ---
	FOR
	TO
	DO
	DONE
	BEGIN
	END

	tokenCount
)

var tokenNames = [...]string{
	EOF: "EOF", ERROR: "ERROR",
	IDENT: "IDENT", TYVAR: "TYVAR", CONSTRUCTOR: "CONSTRUCTOR", POLYVAR: "POLYVAR",
	INT: "INT", FLOAT: "FLOAT", STRING: "STRING", CHAR: "CHAR",
	LET: "let", REC: "rec", MUTABLE: "mutable", TYPE: "type", MATCH: "match", WITH: "with",
	IF: "if", THEN: "then", ELSE: "else", FUN: "fun", FUNCTION: "function",
	MODULE: "module", OPEN: "open", EXTERN: "extern", IMPORT: "import",
	GOLANG: "golang", GOOP: "goop", AS: "as", WHEN: "when", OF: "of", IN: "in", AND: "and",
	TRUE: "true", FALSE: "false", UNIT: "()", VAL: "val",
	REQUIRES: "requires", RETURNS: "returns", WHERE: "where", PRIVATE: "private",
	WHILE: "while", EXCEPTION: "exception", RAISE: "raise", TRY: "try", FINALLY: "finally",
	EFFECT: "effect", PERFORM: "perform", CLASS: "class", OBJECT: "object", METHOD: "method",
	INHERIT: "inherit", INITIALIZER: "initializer", VIRTUAL: "virtual", CONSTRAINT: "constraint",
	STRUCT: "struct", SIG: "sig", INCLUDE: "include", LAZY: "lazy", ASSERT: "assert",
	FAILWITH: "failwith", MOD: "mod", LAND: "land", LOR: "lor", LXOR: "lxor",
	NEW: "new", REF: "ref",
	GUARD: "guard", IS: "is", PANIC: "panic", NEWTYPE: "newtype", USING: "using",
	LPAREN: "(", RPAREN: ")", LBRACE: "{", RBRACE: "}",
	LBRACKET: "[", RBRACKET: "]", LBRACKETPIPE: "[|", PIPERBRACKET: "|]",
	AT: "@", ARROW: "->", PIPE: "|", EQUALS: "=", SEMI: ";", COLON: ":", COLONEQ: ":=",
	DOT: ".", COMMA: ",", STAR: "*", STARDOT: "*.", PLUS: "+", MINUS: "-", SLASH: "/",
	UNDERSCORE: "_", CONS: "::", QUESTION: "?", PIPEOP: "|>", DIAMOND: "<>",
	NEQ: "!=", EQEQ: "==", LEQ: "<=", GEQ: ">=", LT: "<", GT: ">", CARET: "^",
	LARROW: "<-", AMPAMP: "&&", PIPEPIPE: "||", BANG: "!",
	PLUSDOT: "+.", MINUSDOT: "-.", SLASHDOT: "/.", PERCENT: "%", BACKTICK: "`",
	TILDE: "~",
	CHAN: "chan", GO: "go", MOVE: "move",
	FOR: "for", TO: "to", DO: "do", DONE: "done", BEGIN: "begin", END: "end",
}

func (t TokenType) String() string {
	if int(t) < len(tokenNames) && tokenNames[t] != "" {
		return tokenNames[t]
	}
	return fmt.Sprintf("TokenType(%d)", int(t))
}

func (t TokenType) IsKeyword() bool {
	switch t {
	case LET, REC, MUTABLE, TYPE, MATCH, WITH, IF, THEN, ELSE, FUN, FUNCTION,
		MODULE, OPEN, EXTERN, IMPORT, GOLANG, GOOP, AS, WHEN, OF, IN, AND,
		TRUE, FALSE, UNIT, VAL, REQUIRES, RETURNS, WHERE, PRIVATE,
		WHILE, EXCEPTION, RAISE, TRY, FINALLY, EFFECT, PERFORM,
		CLASS, OBJECT, METHOD, INHERIT, INITIALIZER, VIRTUAL, CONSTRAINT,
		STRUCT, SIG, INCLUDE, LAZY, ASSERT, FAILWITH, MOD, LAND, LOR, LXOR,
		NEW, REF, GUARD, IS, PANIC, NEWTYPE, USING, CHAN, GO, MOVE, NOT,
		FOR, TO, DO, DONE, BEGIN, END:
		return true
	}
	return false
}

// SourceLoc carries file/line/column/offset information.
type SourceLoc struct {
	File   string
	Line   int
	Column int
	Offset int
}

func (l SourceLoc) String() string {
	return fmt.Sprintf("%s:%d:%d", l.File, l.Line, l.Column)
}

// Token is a single lexical token produced by the lexer.
type Token struct {
	Type    TokenType
	Lexeme  string
	Literal any
	Loc     SourceLoc
}

func (t Token) String() string {
	switch t.Type {
	case IDENT, TYVAR, CONSTRUCTOR, POLYVAR:
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
	case "function":
		return FUNCTION
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
	case "true":
		return TRUE
	case "false":
		return FALSE
	case "()":
		return UNIT
	case "val":
		return VAL
	case "requires":
		return REQUIRES
	case "returns":
		return RETURNS
	case "where":
		return WHERE
	case "private":
		return PRIVATE
	case "while":
		return WHILE
	case "exception":
		return EXCEPTION
	case "raise":
		return RAISE
	case "try":
		return TRY
	case "finally":
		return FINALLY
	case "effect":
		return EFFECT
	case "perform":
		return PERFORM
	case "class":
		return CLASS
	case "object":
		return OBJECT
	case "method":
		return METHOD
	case "inherit":
		return INHERIT
	case "initializer":
		return INITIALIZER
	case "virtual":
		return VIRTUAL
	case "constraint":
		return CONSTRAINT
	case "struct":
		return STRUCT
	case "sig":
		return SIG
	case "include":
		return INCLUDE
	case "lazy":
		return LAZY
	case "assert":
		return ASSERT
	case "failwith":
		return FAILWITH
	case "mod":
		return MOD
	case "land":
		return LAND
	case "lor":
		return LOR
	case "lxor":
		return LXOR
	case "new":
		return NEW
	case "ref":
		return REF
	case "guard":
		return GUARD
	case "is":
		return IS
	case "panic":
		return PANIC
	case "newtype":
		return NEWTYPE
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
