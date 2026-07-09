// Package parser implements a recursive-descent parser for Goop.
//
// It handles:
//   - Module declarations with qualified names (dots)
//   - Open statements
//   - Let declarations (top-level and `let … in` expressions)
//   - Type declarations (records, ADTs, aliases)
//   - Extern declarations
//   - Full expression language with proper precedence
//   - Pattern matching (wildcard, identifier, literal, constructor, record, tuple, list, cons, alias)
//   - Pipeline operator, error propagation, match macros
package parser

import (
	"fmt"
	"strings"

	"goop.dev/compiler/internal/ast"
	lc0 "goop.dev/compiler/internal/lexer"
	"goop.dev/compiler/internal/token"
)

// ParseError records a parse failure with source location.
type ParseError struct {
	Msg string
	Loc token.SourceLoc
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s: %s", e.Loc, e.Msg)
}

// Parser holds the state of the recursive-descent parser.
type Parser struct {
	file      string
	src       []byte // raw source bytes for reading inline Go blocks
	tokens    []token.Token
	pos       int // index into tokens slice
	errs      []error
	wherePred bool // true when parsing a where-clause predicate (stops at =)
}

// Parse reads a complete Goop source file and returns the AST module.
func Parse(file string, src []byte) (*ast.Module, error) {
	toks, err := lc0.Lex(file, src)
	if err != nil {
		return nil, err
	}
	p := &Parser{file: file, src: src, tokens: toks}
	mod := p.parseModule()
	if len(p.errs) > 0 {
		// Return the module along with the first error so callers can inspect
		// partial results.
		return mod, p.errs[0]
	}
	return mod, nil
}

// Errors returns all accumulated parse errors.
func (p *Parser) Errors() []error {
	return p.errs
}

// ---------------------------------------------------------------------------
// Low‑level helpers
// ---------------------------------------------------------------------------

func (p *Parser) cur() token.Token {
	if p.pos >= len(p.tokens) {
		return token.Token{Type: token.EOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) peek() token.Token {
	if p.pos+1 >= len(p.tokens) {
		return token.Token{Type: token.EOF}
	}
	return p.tokens[p.pos+1]
}

func (p *Parser) advance() token.Token {
	t := p.cur()
	if t.Type != token.EOF {
		p.pos++
	}
	return t
}

func (p *Parser) match(t token.TokenType) bool {
	if p.cur().Type == t {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) expect(t token.TokenType) (token.Token, error) {
	if p.cur().Type == t {
		return p.advance(), nil
	}
	err := p.errorf("expected %s, got %s", t, p.cur().Type)
	return p.cur(), err
}

func (p *Parser) errorf(format string, args ...any) error {
	err := &ParseError{
		Msg: fmt.Sprintf(format, args...),
		Loc: p.cur().Loc,
	}
	p.errs = append(p.errs, err)
	return err
}

// parseGoBlock parses a `go { … }` block inside an extern declaration.
// It reads the raw Go source text between braces, uses source byte offsets
// to advance the parser position past all tokens inside the block INCLUDING
// the closing '}', and returns the Go code content (without surrounding braces).
func (p *Parser) parseGoBlock() string {
	braceTok, _ := p.expect(token.LBRACE)
	if braceTok.Type != token.LBRACE {
		return ""
	}
	startOffset := braceTok.Loc.Offset

	// Read raw source bytes and get the byte offset of the closing '}'.
	code, endOffset := p.readRawGoBlock(startOffset)

	// Advance parser position past all tokens whose source offset is
	// inside the go block, consuming the closing '}' token too.
	for p.pos < len(p.tokens) {
		tok := p.cur()
		if tok.Type == token.EOF {
			break
		}
		if tok.Loc.Offset > endOffset {
			// We've consumed the closing '}'. Stop.
			break
		}
		p.advance()
	}

	return code
}

// readRawGoBlock reads raw source bytes from just after '{' until the matching '}'.
// It counts brace depth in the raw source, skipping string literals and comments.
// Returns the Go code content (trimmed) and the byte offset of the closing '}'.
func (p *Parser) readRawGoBlock(braceOffset int) (string, int) {
	if braceOffset >= len(p.src) || p.src[braceOffset] != '{' {
		p.errorf("internal: readRawGoBlock called without opening brace")
		return "", braceOffset + 1
	}
	pos := braceOffset + 1 // skip '{'
	depth := 1
	for pos < len(p.src) && depth > 0 {
		switch p.src[pos] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				// Don't advance past the matching '}'
				content := string(p.src[braceOffset+1 : pos])
				return strings.TrimSpace(content), pos
			}
		case '/':
			// Skip line comments and block comments
			if pos+1 < len(p.src) && p.src[pos+1] == '/' {
				pos += 2
				for pos < len(p.src) && p.src[pos] != '\n' {
					pos++
				}
				continue
			}
			if pos+1 < len(p.src) && p.src[pos+1] == '*' {
				pos += 2
				for pos+1 < len(p.src) && !(p.src[pos] == '*' && p.src[pos+1] == '/') {
					pos++
				}
				pos += 2 // skip */
				continue
			}
		case '"':
			// Skip Go string literal
			pos++
			for pos < len(p.src) && p.src[pos] != '"' {
				if p.src[pos] == '\\' {
					pos++ // skip escaped char
				}
				pos++
			}
		case '\'':
			// Skip Go rune literal
			pos++
			for pos < len(p.src) && p.src[pos] != '\'' {
				if p.src[pos] == '\\' {
					pos++
				}
				pos++
			}
		case '`':
			// Skip Go raw string literal
			pos++
			for pos < len(p.src) && p.src[pos] != '`' {
				pos++
			}
		}
		pos++
	}
	if depth != 0 {
		p.errorf("unterminated go block (missing '}')")
		return "", braceOffset + 1
	}
	content := string(p.src[braceOffset+1 : pos])
	return strings.TrimSpace(content), pos
}

// ---------------------------------------------------------------------------
// Expression / pattern / type start‑of‑input predicates
// ---------------------------------------------------------------------------

func (p *Parser) canStartExpr(t token.Token) bool {
	switch t.Type {
	// Note: token.LET is intentionally excluded from canStartExpr
	// because a `let` at expression level must always be `let … in …`,
	// which is parsed by parsePrefix().  Including LET here causes the
	// function‑application juxtaposition loop to greedily consume a
	// top‑level `let` declaration as if it were an argument.
	case token.IDENT, token.CONSTRUCTOR, token.INT, token.FLOAT,
		token.STRING, token.CHAR, token.TRUE, token.FALSE,
		token.LPAREN, token.LBRACE, token.LBRACKET,
		token.IF, token.MATCH, token.FUN, token.GUARD:
		return true
	default:
		return false
	}
}

func (p *Parser) canStartPattern(t token.Token) bool {
	switch t.Type {
	case token.UNDERSCORE, token.IDENT, token.CONSTRUCTOR,
		token.INT, token.FLOAT, token.STRING, token.CHAR,
		token.TRUE, token.FALSE,
		token.LPAREN, token.LBRACE, token.LBRACKET:
		return true
	default:
		return false
	}
}

func (p *Parser) canStartType(t token.Token) bool {
	switch t.Type {
	case token.IDENT, token.CONSTRUCTOR, token.TYVAR,
		token.LPAREN, token.LBRACE:
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Module
// ---------------------------------------------------------------------------

func (p *Parser) parseModule() *ast.Module {
	mod := &ast.Module{}

	// Optional `module Name`
	if p.cur().Type == token.MODULE {
		p.advance()
		mod.Name = p.parseQualifiedName()
	}

	// Reject legacy `open` with migration hint
	if p.cur().Type == token.OPEN {
		p.errorf("'open' is removed; use `import goop \"path\"` or `import goop . \"path\"` for dot import")
		p.advance()
		p.parseQualifiedName()
	}

	// Zero or more import declarations
	for p.cur().Type == token.IMPORT {
		specs := p.parseImportDecl()
		mod.Imports = append(mod.Imports, specs...)
	}

	// Zero or more top-level declarations
	for p.cur().Type != token.EOF {
		decl := p.parseTopDecl()
		if decl != nil {
			mod.Decls = append(mod.Decls, decl)
		}
	}

	return mod
}

// parseQualifiedName reads a dot-separated identifier chain.
// E.g. "Trading.OrderBook" or "Std.List".
func (p *Parser) parseQualifiedName() string {
	name := ""
	for {
		tok := p.cur()
		if tok.Type == token.CONSTRUCTOR || tok.Type == token.IDENT {
			p.advance()
			name += tok.Lexeme
		} else {
			break
		}
		if p.cur().Type == token.DOT {
			p.advance()
			name += "."
		} else {
			break
		}
	}
	return name
}

// ---------------------------------------------------------------------------
// Top-level declarations
// ---------------------------------------------------------------------------

func (p *Parser) parseTopDecl() ast.TopDecl {
	switch p.cur().Type {
	case token.AT:
		return p.parseGolangEmbedDecl()
	case token.PRIVATE:
		p.advance()
		switch p.cur().Type {
		case token.LET:
			return p.parseLetDecl(true, true)
		case token.TYPE:
			return p.parseTypeDecl(true)
		default:
			p.errorf("expected let or type after private, got %s", p.cur().Type)
			p.advance()
			return nil
		}
	case token.LET:
		return p.parseLetDecl(true, false)
	case token.TYPE:
		return p.parseTypeDecl(false)
	case token.EXTERN:
		p.errorf("'extern' is removed; use `import golang \"path\"` or `import golang \"path\" { val ... }`")
		return p.parseExternDeclSkip()
	case token.IMPORT:
		p.errorf("'import' must appear before any declarations")
		p.parseImportDecl()
		return nil
	case token.MODULE:
		p.errorf("unexpected 'module' after first declaration")
		p.advance()
		p.parseQualifiedName()
		return nil
	case token.OPEN:
		p.errorf("'open' is removed; use `import goop \"path\"` or `import goop . \"path\"`")
		p.advance()
		p.parseQualifiedName()
		return nil
	default:
		p.errorf("unexpected token %s at top level", p.cur().Type)
		p.advance()
		return nil
	}
}

// parseLetDecl parses a top-level `let` (no `in`), or
// an active pattern `let (|Name|_|) (...) = ...`.
func (p *Parser) parseLetDecl(isTopLevel bool, isPrivate bool) *ast.LetDecl {
	p.expect(token.LET)
	decl := &ast.LetDecl{Private: isPrivate}

	// Active pattern: let (|Name|_|) ...
	if p.cur().Type == token.LPAREN && p.peek().Type == token.PIPE {
		p.advance() // consume LPAREN
		p.expect(token.PIPE)
		tok := p.cur()
		if tok.Type != token.CONSTRUCTOR && tok.Type != token.IDENT {
			p.errorf("expected active pattern name, got %s", tok.Type)
		} else {
			p.advance()
		}
		p.expect(token.PIPE)
		p.expect(token.UNDERSCORE)
		p.expect(token.PIPE)
		p.expect(token.RPAREN)

		decl.ActivePattern = true
		// parseBinding expects name first, but we already consumed it.
		// Parse params, return type, and body manually.
		b := ast.LetBinding{Name: tok.Lexeme}
		b.Params = p.parseParams()
		if p.match(token.COLON) {
			b.RetType = p.parseType()
		}
		p.expect(token.EQUALS)
		b.Body = p.parseExpr(0)
		decl.Bindings = append(decl.Bindings, b)
		return decl
	}

	if p.match(token.REC) {
		decl.Rec = true
	}
	if p.match(token.MUTABLE) {
		decl.Mutable = true
	}
	decl.Bindings = append(decl.Bindings, p.parseBinding())
	for p.match(token.AND) {
		decl.Bindings = append(decl.Bindings, p.parseBinding())
	}
	return decl
}

// parseBinding parses `name params… (: type)? = expr`.
func (p *Parser) parseBinding() ast.LetBinding {
	b := ast.LetBinding{}

	tok := p.cur()
	if tok.Type != token.IDENT && tok.Type != token.CONSTRUCTOR {
		p.errorf("expected binding name, got %s", tok.Type)
		if p.cur().Type != token.EOF {
			p.advance()
		}
		return b
	}
	p.advance()
	b.Name = tok.Lexeme

	// Parse parameters (zero or more)
	b.Params = p.parseParams()

	// Optional return type annotation
	if p.match(token.COLON) {
		b.RetType = p.parseType()
		// Optional effect row after return type
		if p.cur().Type == token.WITH {
			b.RetEffects = p.parseEffectRow()
		}
	}

	// Expect `=`
	p.expect(token.EQUALS)

	// Parse body expression
	b.Body = p.parseExpr(0)

	return b
}

// parseParams parses a sequence of function parameters.
func (p *Parser) parseParams() []ast.Param {
	var params []ast.Param
	for {
		switch p.cur().Type {
		case token.IDENT:
			tok := p.advance()
			params = append(params, ast.Param{Name: tok.Lexeme})
		case token.LPAREN:
			// `(name : type)`
			p.advance()
			param := ast.Param{}
			nameTok := p.cur()
			if nameTok.Type == token.IDENT || nameTok.Type == token.CONSTRUCTOR {
				p.advance()
				param.Name = nameTok.Lexeme
			}
			if p.match(token.COLON) {
				param.Type = p.parseType()
			}
			p.expect(token.RPAREN)
			params = append(params, param)
		default:
			return params
		}
	}
}

// ---------------------------------------------------------------------------
// Type declarations
// ---------------------------------------------------------------------------

func (p *Parser) parseTypeDecl(isPrivate bool) ast.TopDecl {
	p.expect(token.TYPE)

	// We handle the first binding inline; `and` produces additional decls.
	// NOTE: `and`-connected type decls are returned one at a time.
	// This matches the current module builder which calls parseTopDecl in a loop.

	var allDecls []*ast.TypeDecl

	for {
		d := p.parseTypeBinding()
		allDecls = append(allDecls, d)
		if !p.match(token.AND) {
			break
		}
	}

	// Return only the first; remaining will be returned on subsequent calls
	// via a side channel.  For the bootstrap we accept this limitation.
	// A production parser would push the extras onto a pending queue.
	if len(allDecls) > 0 {
		allDecls[0].Private = isPrivate
		return allDecls[0]
	}
	return nil
}

func (p *Parser) parseTypeBinding() *ast.TypeDecl {
	d := &ast.TypeDecl{}

	tok := p.cur()
	if tok.Type == token.CONSTRUCTOR || tok.Type == token.IDENT {
		p.advance()
		d.Name = tok.Lexeme
	} else {
		p.errorf("expected type name, got %s", tok.Type)
		return d
	}

	// Optional type parameters: 'a 'b …
	for p.cur().Type == token.TYVAR {
		tv := p.advance()
		d.TypeParams = append(d.TypeParams, tv.Lexeme)
	}

	// Check for `: 1` linear quantity annotation
	// e.g. `type file_handle : 1` or `type file_handle : 1 = ...`
	if p.cur().Type == token.COLON {
		// Peek: if next token is INT, it's a quantity annotation
		if p.peek().Type == token.INT {
			p.advance() // consume COLON
			intTok := p.advance()
			val, ok := intTok.Literal.(int64)
			if ok && val == 1 {
				d.Quantity = 1
			} else {
				p.errorf("expected '1' for linear quantity, got %v", intTok.Literal)
			}
		}
		// If next token is not INT, it might be a type annotation for a
		// different syntax — bail out and let the expected '=' catch it.
	}

	// Opaque linear type: `type T : 1` (no `= ...` body)
	if d.Quantity == 1 && p.cur().Type != token.EQUALS {
		d.Kind = &ast.OpaqueTypeKind{}
		return d
	}

	p.expect(token.EQUALS)

	// Determine the kind of RHS
	switch {
	case p.cur().Type == token.LBRACE:
		d.Kind = p.parseRecordTypeKind()
	case p.cur().Type == token.PIPE:
		d.Kind = p.parseADTTypeKind()
	default:
		// Could be ADT without leading pipe, or a type alias.
		// ADT: starts with CONSTRUCTOR followed by PIPE or EOF/RBRACE/etc.
		// Alias: any other type expression.
		if p.cur().Type == token.CONSTRUCTOR {
			// Peek ahead: if next is PIPE or RBRACE or EOF, it's an ADT.
			// Otherwise it might be a qualified type name (A.B) or alias.
			next := p.peek().Type
			if next == token.PIPE || next == token.EOF || next == token.RBRACE ||
				next == token.LET || next == token.TYPE || next == token.MODULE || next == token.OPEN {
				d.Kind = p.parseADTTypeKindNoPipe()
				return d
			}
		}
		d.Kind = &ast.AliasTypeKind{Alias: p.parseType()}
	}
	return d
}

func (p *Parser) parseRecordTypeKind() *ast.RecordTypeKind {
	p.expect(token.LBRACE)
	rk := &ast.RecordTypeKind{}
	for p.cur().Type != token.RBRACE && p.cur().Type != token.EOF {
		ft := ast.FieldType{}
		tok := p.cur()
		if tok.Type == token.IDENT {
			p.advance()
			ft.Name = tok.Lexeme
		}
		p.expect(token.COLON)
		ft.Type = p.parseType()
		rk.Fields = append(rk.Fields, ft)
		if !p.match(token.SEMI) {
			break
		}
	}
	p.expect(token.RBRACE)
	return rk
}

func (p *Parser) parseADTTypeKind() *ast.ADTTypeKind {
	// Leading pipe already matched or present
	if p.cur().Type == token.PIPE {
		p.advance()
	}
	return p.parseADTTypeKindNoPipe()
}

func (p *Parser) parseADTTypeKindNoPipe() *ast.ADTTypeKind {
	ak := &ast.ADTTypeKind{}
	for {
		c := ast.ADTCase{}
		tok := p.cur()
		if tok.Type != token.CONSTRUCTOR {
			break
		}
		p.advance()
		c.Name = tok.Lexeme

		if p.match(token.OF) {
			c.Arg = p.parseType()
		}
		ak.Cases = append(ak.Cases, c)

		if !p.match(token.PIPE) {
			break
		}
	}
	return ak
}

// ---------------------------------------------------------------------------
// @golang embedded Go
// ---------------------------------------------------------------------------

func (p *Parser) parseGolangEmbedDecl() *ast.GolangEmbedDecl {
	p.expect(token.AT)
	nameTok := p.cur()
	if nameTok.Type != token.IDENT && nameTok.Type != token.CONSTRUCTOR && nameTok.Type != token.GOLANG {
		p.errorf("expected attribute name after @, got %s", nameTok.Type)
		return nil
	}
	if nameTok.Lexeme != "golang" && nameTok.Type != token.GOLANG {
		p.errorf("unknown attribute @%s (expected @golang)", nameTok.Lexeme)
		return nil
	}
	p.advance()
	decl := &ast.GolangEmbedDecl{}
	decl.GoCode = p.parseGoBlock()
	for p.cur().Type == token.VAL {
		p.advance()
		ev := ast.ExternVal{}
		nameTok := p.cur()
		if nameTok.Type == token.IDENT || nameTok.Type == token.CONSTRUCTOR {
			p.advance()
			ev.Name = nameTok.Lexeme
		} else {
			p.errorf("expected val binding name, got %s", nameTok.Type)
			continue
		}
		p.expect(token.COLON)
		ev.Type = p.parseType()
		decl.Vals = append(decl.Vals, ev)
	}
	return decl
}

// ---------------------------------------------------------------------------
// Import declarations
// ---------------------------------------------------------------------------

func (p *Parser) parseImportDecl() []ast.ImportSpec {
	p.expect(token.IMPORT)
	if p.cur().Type == token.LPAREN {
		p.advance()
		var specs []ast.ImportSpec
		for p.cur().Type != token.RPAREN && p.cur().Type != token.EOF {
			specs = append(specs, p.parseImportSpec())
		}
		p.expect(token.RPAREN)
		return specs
	}
	return []ast.ImportSpec{p.parseImportSpec()}
}

func (p *Parser) parseImportSpec() ast.ImportSpec {
	spec := ast.ImportSpec{}

	// Optional local alias before kind keyword
	if p.cur().Type == token.IDENT {
		spec.Alias = p.cur().Lexeme
		p.advance()
	}

	switch p.cur().Type {
	case token.GOLANG:
		spec.Kind = ast.ImportGolang
		p.advance()
	case token.GOOP:
		spec.Kind = ast.ImportGoop
		p.advance()
		// Dot import: c0 . "path"
		if p.cur().Type == token.DOT {
			if spec.Alias != "" {
				p.errorf("dot import cannot have a local alias; use `import goop . \"path\"`")
			}
			spec.Alias = "."
			p.advance()
		}
	case token.GO:
		p.errorf("use `golang` for Go package imports (`go` is reserved for concurrency)")
		p.advance()
		if p.cur().Type == token.STRING {
			p.advance()
		}
		return spec
	default:
		p.errorf("expected `golang` or `goop` after import, got %s", p.cur().Type)
		p.synchronizeImportSpec()
		return spec
	}

	tok := p.cur()
	if tok.Type == token.STRING {
		p.advance()
		spec.Path = tok.Lexeme
	} else {
		p.errorf("expected string literal import path, got %s", tok.Type)
	}

	if p.cur().Type == token.LBRACE {
		if spec.Kind == ast.ImportGoop {
			p.errorf("c0 imports cannot have `{ val ... }` blocks (only golang imports)")
		}
		p.advance()
		for p.cur().Type != token.RBRACE && p.cur().Type != token.EOF {
			if p.match(token.VAL) {
				ev := ast.ExternVal{}
				nameTok := p.cur()
				if nameTok.Type == token.IDENT || nameTok.Type == token.CONSTRUCTOR {
					p.advance()
					ev.Name = nameTok.Lexeme
				} else {
					p.errorf("expected import binding name, got %s", nameTok.Type)
				}
				p.expect(token.COLON)
				ev.Type = p.parseType()
				spec.Vals = append(spec.Vals, ev)
			} else {
				p.errorf("expected `val` inside golang import block, got %s", p.cur().Type)
				p.advance()
			}
		}
		p.expect(token.RBRACE)
	}

	return spec
}

func (p *Parser) synchronizeImportSpec() {
	for p.cur().Type != token.EOF {
		switch p.cur().Type {
		case token.RPAREN, token.GOLANG, token.GOOP, token.IMPORT:
			return
		default:
			p.advance()
		}
	}
}

// ---------------------------------------------------------------------------
// Extern declarations (legacy skip — parser rejects with migration hint)
// ---------------------------------------------------------------------------

func (p *Parser) parseExternDeclSkip() ast.TopDecl {
	p.expect(token.EXTERN)
	if p.cur().Type == token.STRING {
		p.advance()
	}
	if p.cur().Type == token.STRING {
		p.advance()
	}
	if p.cur().Type == token.LBRACE {
		depth := 1
		p.advance()
		for depth > 0 && p.cur().Type != token.EOF {
			switch p.cur().Type {
			case token.LBRACE:
				depth++
			case token.RBRACE:
				depth--
			}
			p.advance()
		}
	}
	return nil
}

func (p *Parser) parseExternDecl() ast.TopDecl {
	p.expect(token.EXTERN)
	ed := &ast.ExternDecl{}

	// First string: language
	tok := p.cur()
	if tok.Type == token.STRING {
		p.advance()
		ed.Lang = tok.Lexeme
	} else {
		p.errorf("expected string literal for extern language, got %s", tok.Type)
	}

	// Second string: import path
	tok = p.cur()
	if tok.Type == token.STRING {
		p.advance()
		ed.Path = tok.Lexeme
	} else {
		p.errorf("expected string literal for extern path, got %s", tok.Type)
	}

	p.expect(token.LBRACE)
	for p.cur().Type != token.RBRACE && p.cur().Type != token.EOF {
		if p.match(token.GO) {
			if p.cur().Type == token.LBRACE {
				p.errorf("use @golang { ... } for embedded Go code (go { } inside extern is removed)")
				p.parseGoBlock()
			} else {
				p.errorf("unexpected 'go' inside extern block")
				p.advance()
			}
		} else if p.match(token.VAL) {
			ev := ast.ExternVal{}
			nameTok := p.cur()
			if nameTok.Type == token.IDENT || nameTok.Type == token.CONSTRUCTOR {
				p.advance()
				ev.Name = nameTok.Lexeme
			} else {
				p.errorf("expected extern binding name, got %s", nameTok.Type)
			}
			p.expect(token.COLON)
			ev.Type = p.parseType()
			ed.Vals = append(ed.Vals, ev)
		} else {
			p.errorf("expected 'go' or 'val' inside extern block, got %s", p.cur().Type)
			p.advance()
		}
	}
	p.expect(token.RBRACE)
	return ed
}

// ---------------------------------------------------------------------------
// Expression parsing (Pratt / precedence-climbing)
// ---------------------------------------------------------------------------

// Precedence levels (higher = binds tighter).
const (
	precLowest  = 1
	precPipe    = 2 // |>
	precOr      = 3 // ||
	precAnd     = 4 // &&
	precCompare = 5 // == != < > <= >= <>
	precCons    = 6 // ::  (right-associative)
	precAdd     = 7 // + - ^
	precMul     = 8 // * / *.
	precApp     = 9 // function application (juxtaposition)
	precUnary   = 10
	precPostfix = 11 // . (field), ? (propagation)
)

func (p *Parser) precedence(op token.TokenType) int {
	// In where-clause predicates, = is NOT a comparison operator — it's the
	// binding separator that follows the type annotation.
	if p.wherePred && op == token.EQUALS {
		return 0
	}
	switch op {
	case token.PIPEOP:
		return precPipe
	case token.PIPEPIPE:
		return precOr
	case token.AMPAMP:
		return precAnd
	case token.EQEQ, token.NEQ, token.EQUALS, token.LT, token.GT, token.LEQ, token.GEQ, token.DIAMOND:
		return precCompare
	case token.CONS:
		return precCons
	case token.PLUS, token.MINUS, token.CARET, token.PLUSDOT, token.MINUSDOT:
		return precAdd
	case token.STAR, token.SLASH, token.STARDOT, token.SLASHDOT, token.PERCENT:
		return precMul
	default:
		return 0
	}
}

func (p *Parser) isRightAssoc(op token.TokenType) bool {
	return op == token.CONS
}

// parseExpr is the top-level expression parser, starting at the given
// minimum precedence.
func (p *Parser) parseExpr(minPrec int) ast.Expr {
	left := p.parsePrefix()
	// Track line of the leftmost expression for offside rule.
	leftLine := p.cur().Loc.Line
	if p.pos > 0 {
		leftLine = p.tokens[p.pos-1].Loc.Line
	}

	for {
		cur := p.cur()

		// Postfix operators (bind tighter than juxtaposition).
		// This ensures `o.price` is parsed before `greater o.price`.
		switch cur.Type {
		case token.QUESTION:
			if precPostfix < minPrec {
				return left
			}
			left = p.parseQuestionExpr(left)
			continue
		case token.DOT:
			if precPostfix < minPrec {
				return left
			}
			left = p.parseFieldAccessExpr(left)
			continue
		case token.IS:
			if precCompare < minPrec {
				return left
			}
			left = p.parseIsExpr(left)
			continue
		case token.AS:
			if precCompare < minPrec {
				return left
			}
			left = p.parseAsMatchExpr(left)
			continue
		}

		// Function application (juxtaposition) — highest precedence
		// after postfix. Only apply when on the same line (offside rule).
		if p.canStartExpr(cur) && precApp >= minPrec && cur.Loc.Line == leftLine {
			appLoc := cur.Loc // location of the argument start
			arg := p.parsePrefix()
			left = &ast.AppExpr{Func: left, Arg: arg, Loc: appLoc}
			leftLine = p.tokens[p.pos-1].Loc.Line
			continue
		}

		// Binary operators
		prec := p.precedence(cur.Type)
		if prec == 0 || prec < minPrec {
			break
		}

		op := cur.Type
		p.advance()

		nextMinPrec := prec
		if !p.isRightAssoc(op) {
			nextMinPrec = prec + 1
		}
		right := p.parseExpr(nextMinPrec)
		left = &ast.BinaryExpr{Left: left, Op: op, Right: right, Loc: cur.Loc}
	}

	return left
}

// applyPostfix applies field access (.) to an atom.  This ensures
// that `o.price` is parsed as a unit before juxtaposition.
// The `?` operator is NOT applied here because it binds to the full
// preceding expression, not just the atom.
func (p *Parser) applyPostfix(left ast.Expr) ast.Expr {
	for {
		switch p.cur().Type {
		case token.DOT:
			left = p.parseFieldAccessExpr(left)
		default:
			return left
		}
	}
}

// parsePrefix handles prefix expressions: atoms, unary minus, let-in, if,
// match, fun, guard.
func (p *Parser) parsePrefix() ast.Expr {
	cur := p.cur()

	switch cur.Type {
	// --- literals ---
	case token.INT, token.FLOAT, token.STRING, token.CHAR:
		tok := p.advance()
		return &ast.LitExpr{Value: tok.Literal, Kind: tok.Type, Loc: tok.Loc}
	case token.TRUE:
		tok := p.advance()
		return &ast.LitExpr{Value: true, Kind: token.TRUE, Loc: tok.Loc}
	case token.FALSE:
		tok := p.advance()
		return &ast.LitExpr{Value: false, Kind: token.FALSE, Loc: tok.Loc}

	// --- identifiers ---
	case token.IDENT:
		tok := p.advance()
		// Check for computation expression: builder { ... }
		if isBuilder(tok.Lexeme) && p.cur().Type == token.LBRACE {
			return p.parseCompExpr(tok.Loc, tok.Lexeme)
		}
		// Check for select { ... }
		if tok.Lexeme == "select" && p.cur().Type == token.LBRACE {
			return p.parseSelectExpr(tok.Loc)
		}
		return p.applyPostfix(&ast.IdentExpr{Name: tok.Lexeme, Loc: tok.Loc})

	// --- concurrency ---
	case token.GO:
		goTok := p.advance()
		return &ast.GoExpr{Expr: p.parseExpr(0), Loc: goTok.Loc}

	case token.USING:
		usingTok := p.advance()
		pat := p.parseSimplePattern()
		p.expect(token.EQUALS)
		expr := p.parseExpr(0)
		p.expect(token.IN)
		body := p.parseExpr(0)
		return &ast.UsingExpr{Pattern: pat, Expr: expr, Body: body, Loc: usingTok.Loc}

	case token.CONSTRUCTOR:
		tok := p.advance()
		ce := &ast.ConstructorExpr{Name: tok.Lexeme, Loc: tok.Loc}
		if p.canStartExpr(p.cur()) {
			ce.Arg = p.parsePrefix()
		}
		return p.applyPostfix(ce)

	// --- grouped / tuple / list ---
	case token.LPAREN:
		return p.parseParenOrTupleExpr()
	case token.LBRACE:
		return p.parseRecordOrUpdateExpr()
	case token.LBRACKET:
		return p.parseListExpr()

	// --- keyword expressions ---
	case token.IF:
		return p.parseIfExpr()
	case token.MATCH:
		return p.parseMatchExpr()
	case token.LET:
		return p.parseLetInExpr()
	case token.FUN:
		return p.parseFunExpr()
	case token.GUARD:
		return p.parseGuardExpr()

	// --- unary minus ---
	case token.MINUS:
		minusTok := p.advance()
		operand := p.parseExpr(precUnary)
		// Desugar `-x` as `0 - x`
		return &ast.BinaryExpr{
			Left:  &ast.LitExpr{Value: int64(0), Kind: token.INT},
			Op:    token.MINUS,
			Right: operand,
			Loc:   minusTok.Loc,
		}

	// --- logical not ---
	case token.NOT:
		notTok := p.advance()
		operand := p.parseExpr(precUnary)
		// Emit as `!operand` — represented as a special binary or unary.
		// For now, desugar to `if operand then false else true`
		return &ast.IfExpr{
			Cond:       operand,
			ThenBranch: &ast.LitExpr{Value: false, Kind: token.FALSE},
			ElseBranch: &ast.LitExpr{Value: true, Kind: token.TRUE},
			Loc:        notTok.Loc,
		}
	}

	p.errorf("unexpected token %s in expression", cur.Type)
	p.advance()
	return &ast.LitExpr{Value: nil, Kind: token.UNIT}
}

// ---------------------------------------------------------------------------
// Expression sub-parsers
// ---------------------------------------------------------------------------

func (p *Parser) parseParenOrTupleExpr() ast.Expr {
	lparenLoc := p.cur().Loc
	p.expect(token.LPAREN)
	// Unit literal
	if p.match(token.RPAREN) {
		return &ast.LitExpr{Value: nil, Kind: token.UNIT, Loc: lparenLoc}
	}
	first := p.parseExpr(0)
	if p.match(token.RPAREN) {
		return &ast.ParenExpr{Inner: first, Loc: lparenLoc}
	}
	// Tuple
	elems := []ast.Expr{first}
	for p.match(token.COMMA) {
		elems = append(elems, p.parseExpr(0))
	}
	p.expect(token.RPAREN)
	return &ast.TupleExpr{Elems: elems, Loc: lparenLoc}
}

func (p *Parser) parseRecordOrUpdateExpr() ast.Expr {
	lbraceLoc := p.cur().Loc
	p.expect(token.LBRACE)

	// Save position for backtracking
	savePos := p.pos
	saveErrs := len(p.errs)

	// Try parsing an expression
	var firstExpr ast.Expr
	if p.canStartExpr(p.cur()) {
		firstExpr = p.parseExpr(0)
	}

	if p.match(token.WITH) {
		// Record update: { expr with field = value; ... }
		fields := p.parseRecordFields()
		p.expect(token.RBRACE)
		return &ast.RecordUpdateExpr{Base: firstExpr, Fields: fields, Loc: lbraceLoc}
	}

	// Not a record update.  Backtrack and parse as record literal.
	// We consumed an expression that is actually the first field name.
	// E.g. `{ id = 42; ... }` → parseExpr returned IdentExpr("id"),
	// and now the next token is `=`.
	//
	// `{ x; y }` → parseExpr returned IdentExpr("x"), next is `;` or `}`.

	p.pos = savePos
	p.errs = p.errs[:saveErrs]

	fields := p.parseRecordFields()
	p.expect(token.RBRACE)
	return &ast.RecordExpr{Fields: fields, Loc: lbraceLoc}
}

// parseRecordFields parses `name = expr` or `name` (punning), separated by `;`.
func (p *Parser) parseRecordFields() []ast.RecordField {
	var fields []ast.RecordField
	for p.cur().Type != token.RBRACE && p.cur().Type != token.EOF {
		f := ast.RecordField{}
		tok := p.cur()
		if tok.Type != token.IDENT {
			p.errorf("expected field name, got %s", tok.Type)
			break
		}
		p.advance()
		f.Name = tok.Lexeme

		if p.match(token.EQUALS) {
			f.Value = p.parseExpr(0)
		}
		fields = append(fields, f)

		if !p.match(token.SEMI) {
			break
		}
	}
	return fields
}

func (p *Parser) parseListExpr() ast.Expr {
	lbrackLoc := p.cur().Loc
	p.expect(token.LBRACKET)
	if p.match(token.RBRACKET) {
		return &ast.ListExpr{Elems: nil, Loc: lbrackLoc}
	}
	var elems []ast.Expr
	elems = append(elems, p.parseExpr(0))
	for p.match(token.SEMI) {
		elems = append(elems, p.parseExpr(0))
	}
	p.expect(token.RBRACKET)
	return &ast.ListExpr{Elems: elems, Loc: lbrackLoc}
}

func (p *Parser) parseIfExpr() ast.Expr {
	ifLoc := p.cur().Loc
	p.expect(token.IF)
	cond := p.parseExpr(0)
	p.expect(token.THEN)
	thenBranch := p.parseExpr(0)
	p.expect(token.ELSE)
	elseBranch := p.parseExpr(0)
	return &ast.IfExpr{Cond: cond, ThenBranch: thenBranch, ElseBranch: elseBranch, Loc: ifLoc}
}

func (p *Parser) parseMatchExpr() ast.Expr {
	matchLoc := p.cur().Loc
	p.expect(token.MATCH)
	scrutinee := p.parseExpr(0)
	p.expect(token.WITH)
	// Optional leading `|`
	p.match(token.PIPE)

	arms := p.parseMatchArms()
	return &ast.MatchExpr{Scrutinee: scrutinee, Arms: arms, Loc: matchLoc}
}

func (p *Parser) parseMatchArms() []ast.MatchArm {
	var arms []ast.MatchArm
	for p.cur().Type != token.EOF && p.canStartPattern(p.cur()) {
		arm := ast.MatchArm{}
		arm.Pattern = p.parsePattern()

		if p.match(token.WHEN) {
			arm.Guard = p.parseExpr(0)
		}
		p.expect(token.ARROW)
		arm.Body = p.parseExpr(0)
		arms = append(arms, arm)

		if !p.match(token.PIPE) {
			break
		}
		// Allow trailing pipe at EOF or dedent
		if !p.canStartPattern(p.cur()) && p.cur().Type != token.PIPE {
			break
		}
	}
	return arms
}

func (p *Parser) parseLetInExpr() ast.Expr {
	letLoc := p.cur().Loc
	decl := p.parseLetDecl(false, false)
	// `in` is optional — when absent the offside rule treats the next
	// expression at the same or greater indentation as the body.
	p.match(token.IN)
	body := p.parseExpr(0)
	return &ast.LetInExpr{Mutable: decl.Mutable, Bindings: decl.Bindings, Body: body, Loc: letLoc}
}

func (p *Parser) parseFunExpr() ast.Expr {
	funLoc := p.cur().Loc
	p.expect(token.FUN)
	params := p.parseParams()
	p.expect(token.ARROW)
	body := p.parseExpr(0)
	return &ast.FunExpr{Params: params, Body: body, Loc: funLoc}
}

func (p *Parser) parseGuardExpr() ast.Expr {
	guardLoc := p.cur().Loc
	p.expect(token.GUARD)
	ge := &ast.GuardExpr{Loc: guardLoc}
	// Parse first binding
	pat := p.parsePattern()
	p.expect(token.EQUALS)
	rh := p.parseExpr(0)
	ge.Bindings = append(ge.Bindings, ast.GuardBinding{Pattern: pat, Expr: rh})
	// Parse additional `and` bindings
	for p.match(token.AND) {
		pat2 := p.parsePattern()
		p.expect(token.EQUALS)
		rh2 := p.parseExpr(0)
		ge.Bindings = append(ge.Bindings, ast.GuardBinding{Pattern: pat2, Expr: rh2})
	}
	p.expect(token.ELSE)
	ge.Else_ = p.parseExpr(0)
	return ge
}

// Postfix / special operators

func (p *Parser) parseQuestionExpr(left ast.Expr) ast.Expr {
	qLoc := p.cur().Loc
	qLine := qLoc.Line // line of the `?` token
	p.advance()        // consume ?
	qe := &ast.QuestionExpr{Left: left, Loc: qLoc}
	// Optional argument: only when on the same line (offside rule).
	// Valid forms:  expr ?              (bare)
	//               expr ? "message"    (string)
	//               expr ? fun e -> ... (transform)
	if (p.cur().Type == token.STRING || p.cur().Type == token.FUN) && p.cur().Loc.Line == qLine {
		qe.Arg = p.parseExpr(precPostfix)
	}
	return qe
}

func (p *Parser) parseFieldAccessExpr(left ast.Expr) ast.Expr {
	dotLoc := p.cur().Loc
	p.advance() // consume .
	tok := p.cur()
	if tok.Type == token.IDENT || tok.Type == token.CONSTRUCTOR {
		p.advance()
		return &ast.FieldAccessExpr{Left: left, Field: tok.Lexeme, Loc: dotLoc}
	}
	p.errorf("expected field name after '.', got %s", tok.Type)
	return left
}

func (p *Parser) parseIsExpr(left ast.Expr) ast.Expr {
	isLoc := p.cur().Loc
	p.advance() // consume is
	pat := p.parsePattern()
	return &ast.IsExpr{Left: left, Pattern: pat, Loc: isLoc}
}

func (p *Parser) parseAsMatchExpr(left ast.Expr) ast.Expr {
	asLoc := p.cur().Loc
	p.advance() // consume as
	pat := p.parsePattern()
	p.expect(token.ARROW)
	body := p.parseExpr(4)
	p.expect(token.ELSE)
	elseBody := p.parseExpr(4)
	return &ast.AsMatchExpr{Left: left, Pattern: pat, Body: body, ElseBody: elseBody, Loc: asLoc}
}

// ---------------------------------------------------------------------------
// Pattern parsing
// ---------------------------------------------------------------------------

// parsePattern parses a full pattern, including `::` (cons) and `as` (alias).
func (p *Parser) parsePattern() ast.Pattern {
	left := p.parseSimplePattern()

	// `::` cons pattern (right-associative)
	if p.cur().Type == token.CONS {
		p.advance()
		right := p.parsePattern()
		return &ast.ConsPattern{Head: left, Tail: right}
	}

	// `as` alias pattern
	if p.match(token.AS) {
		tok := p.cur()
		if tok.Type == token.IDENT {
			p.advance()
			return &ast.AliasPattern{Pattern: left, Name: tok.Lexeme}
		}
		p.errorf("expected identifier after 'as', got %s", tok.Type)
	}

	return left
}

// parseSimplePattern parses a pattern atom, including constructor application.
func (p *Parser) parseSimplePattern() ast.Pattern {
	cur := p.cur()
	switch cur.Type {
	case token.UNDERSCORE:
		p.advance()
		return &ast.WildcardPattern{}

	case token.IDENT:
		tok := p.advance()
		return &ast.IdentPattern{Name: tok.Lexeme}

	case token.CONSTRUCTOR:
		tok := p.advance()
		cp := &ast.ConstructorPattern{Name: tok.Lexeme}
		if p.canStartPattern(p.cur()) {
			cp.Arg = p.parseSimplePattern()
		}
		return cp

	case token.INT, token.FLOAT, token.STRING, token.CHAR, token.TRUE, token.FALSE:
		tok := p.advance()
		return &ast.LitPattern{Value: tok.Literal, Kind: tok.Type}

	case token.LPAREN:
		return p.parseParenOrTuplePattern()
	case token.LBRACE:
		return p.parseRecordPattern()
	case token.LBRACKET:
		return p.parseListPattern()

	default:
		p.errorf("unexpected token %s in pattern", cur.Type)
		p.advance()
		return &ast.WildcardPattern{}
	}
}

func (p *Parser) parseParenOrTuplePattern() ast.Pattern {
	p.expect(token.LPAREN)
	first := p.parsePattern()
	if p.match(token.RPAREN) {
		return first // parenthesized
	}
	// Tuple pattern
	elems := []ast.Pattern{first}
	for p.match(token.COMMA) {
		elems = append(elems, p.parsePattern())
	}
	p.expect(token.RPAREN)
	return &ast.TuplePattern{Elems: elems}
}

func (p *Parser) parseRecordPattern() ast.Pattern {
	p.expect(token.LBRACE)
	rp := &ast.RecordPattern{}
	for p.cur().Type != token.RBRACE && p.cur().Type != token.EOF {
		f := ast.RecordPatField{}
		tok := p.cur()
		if tok.Type == token.IDENT {
			p.advance()
			f.Name = tok.Lexeme
		} else {
			p.errorf("expected field name in record pattern, got %s", tok.Type)
			break
		}
		if p.match(token.EQUALS) {
			f.Pattern = p.parsePattern()
		}
		rp.Fields = append(rp.Fields, f)
		if !p.match(token.SEMI) {
			break
		}
	}
	p.expect(token.RBRACE)
	return rp
}

func (p *Parser) parseListPattern() ast.Pattern {
	p.expect(token.LBRACKET)
	if p.match(token.RBRACKET) {
		return &ast.ListPattern{Elems: nil}
	}
	var elems []ast.Pattern
	elems = append(elems, p.parsePattern())
	for p.match(token.SEMI) {
		elems = append(elems, p.parsePattern())
	}
	p.expect(token.RBRACKET)
	return &ast.ListPattern{Elems: elems}
}

// ---------------------------------------------------------------------------
// Type parsing
// ---------------------------------------------------------------------------

// parseType parses a type expression.
// Arrow `->` is right-associative and lowest precedence.
// After parsing a function type, an optional `with { ... }` effect row is parsed.
func (p *Parser) parseType() ast.Type {
	left := p.parseTupleType()
	if p.match(token.ARROW) {
		right := p.parseType() // right-associative
		fun := &ast.TFun{From: left, To: right}
		// Optional effect row: `with { io; log }` or `with { e | .. }`
		if p.cur().Type == token.WITH {
			fun.Effects = p.parseEffectRow()
		}
		return fun
	}
	return left
}

// parseEffectRow parses `with { io; log; e | .. }`.
func (p *Parser) parseEffectRow() *ast.EffectRowType {
	p.expect(token.WITH)
	p.expect(token.LBRACE)

	row := &ast.EffectRowType{}

	for p.cur().Type != token.RBRACE && p.cur().Type != token.EOF {
		// Check for row extension: `| ..` or `e | ..`
		if p.cur().Type == token.PIPE {
			p.advance()
			if p.cur().Type == token.DOT && p.peek().Type == token.DOT {
				p.advance() // first .
				p.advance() // second .
				row.Open = true
				break
			}
			p.errorf("expected '..' after '|' in effect row")
			break
		}

		tok := p.cur()
		if tok.Type == token.IDENT || tok.Type == token.TYVAR {
			p.advance()
			// Check if this is a row variable before `|`
			if p.cur().Type == token.PIPE {
				row.Rest = tok.Lexeme
				row.Open = true
				// Expect pipe and `..`
				// Already consumed; the next iteration will handle it
				continue
			}
			row.Effects = append(row.Effects, tok.Lexeme)
		} else {
			p.errorf("expected effect name or type variable in effect row, got %s", tok.Type)
			break
		}

		if !p.match(token.SEMI) {
			break
		}
	}

	p.expect(token.RBRACE)
	return row
}

// parseTupleType parses `A * B * C`.
func (p *Parser) parseTupleType() ast.Type {
	left := p.parseAppType()
	for p.match(token.STAR) {
		right := p.parseAppType()
		if tup, ok := left.(*ast.TTuple); ok {
			tup.Elems = append(tup.Elems, right)
			left = tup
		} else {
			left = &ast.TTuple{Elems: []ast.Type{left, right}}
		}
	}
	return left
}

// parseAppType parses type application: `A B` where B is the type constructor
// and A is the argument.  E.g. `order list` means `list(order)`.
//
// In ML syntax, the LAST type name is the constructor and everything before
// is the argument (or a tuple of arguments).  We accumulate primaries and
// fold them: [A, B, C] → App(C, App(B, A)).
func (p *Parser) parseAppType() ast.Type {
	var primaries []ast.Type
	primaries = append(primaries, p.parsePrimaryType())
	for p.canStartType(p.cur()) {
		// Peek ahead to avoid consuming `=` or `*` or `->` as type atoms.
		// A type variable 'a followed by `=` starts a type binding, not
		// type application.
		next := p.peek()
		if p.cur().Type == token.TYVAR && (next.Type == token.EQUALS || next.Type == token.EOF || next.Type == token.PIPE) {
			break
		}
		// Stop if the next token is a type delimiter
		if next.Type == token.EQUALS || next.Type == token.ARROW ||
			next.Type == token.RBRACE || next.Type == token.RPAREN ||
			next.Type == token.PIPE || next.Type == token.SEMI {
			// But keep going if the current token is the last primary
			primaries = append(primaries, p.parsePrimaryType())
			break
		}
		primaries = append(primaries, p.parsePrimaryType())
	}

	// Fold: [A, B, C] → App(C, App(B, A))
	result := primaries[0]
	for i := 1; i < len(primaries); i++ {
		result = &ast.TApp{Func: primaries[i], Arg: result}
	}
	// Postfix `chan`: `int chan` → TChan(Elem=int)
	if p.cur().Type == token.CHAN {
		p.advance()
		return &ast.TChan{Elem: result}
	}
	// Postfix `where`: `int where x > 0` → RefinementType{Inner: int, Pred: x > 0}
	if p.match(token.WHERE) {
		p.wherePred = true
		pred := p.parseExpr(0)
		p.wherePred = false
		return &ast.RefinementType{Inner: result, Pred: pred}
	}
	return result
}

// parsePrimaryType parses a type atom.
func (p *Parser) parsePrimaryType() ast.Type {
	cur := p.cur()
	switch cur.Type {
	case token.IDENT, token.CONSTRUCTOR:
		tok := p.advance()
		return &ast.TIdent{Name: tok.Lexeme}
	case token.TYVAR:
		tok := p.advance()
		return &ast.TVar{Name: tok.Lexeme}
	case token.LPAREN:
		p.advance()
		if p.match(token.RPAREN) {
			return &ast.TIdent{Name: "unit"}
		}
		inner := p.parseType()
		if p.match(token.RPAREN) {
			return inner // parenthesized
		}
		// Tuple type
		elems := []ast.Type{inner}
		for p.match(token.COMMA) {
			elems = append(elems, p.parseType())
		}
		p.expect(token.RPAREN)
		return &ast.TTuple{Elems: elems}
	case token.LBRACE:
		p.advance()
		rt := &ast.TRecord{}
		for p.cur().Type != token.RBRACE && p.cur().Type != token.EOF {
			// Check for row extension: | ..
			if p.cur().Type == token.PIPE {
				p.advance()
				if p.cur().Type == token.DOT && p.peek().Type == token.DOT {
					p.advance() // first .
					p.advance() // second .
					rt.Open = true
					break
				}
				p.errorf("expected '..' after '|' in record type")
				break
			}
			ft := ast.FieldType{}
			tok := p.cur()
			if tok.Type == token.IDENT {
				p.advance()
				ft.Name = tok.Lexeme
			}
			p.expect(token.COLON)
			ft.Type = p.parseType()
			rt.Fields = append(rt.Fields, ft)
			if !p.match(token.SEMI) {
				break
			}
		}
		// Check for row extension: | ..
		if p.cur().Type == token.PIPE {
			p.advance()
			if p.cur().Type == token.DOT && p.peek().Type == token.DOT {
				p.advance()
				p.advance()
				rt.Open = true
			} else {
				p.errorf("expected '..' after '|' in record type")
			}
		}
		p.expect(token.RBRACE)
		return rt
	default:
		p.errorf("unexpected token %s in type", cur.Type)
		p.advance()
		return &ast.TIdent{Name: "<error>"}
	}
}

// isBuilder reports whether a name is a known computation expression builder.
func isBuilder(name string) bool {
	switch name {
	case "result", "async", "region":
		return true
	}
	return false
}

// parseSelectExpr parses `select { case x = expr -> body ... default -> body }`.
func (p *Parser) parseSelectExpr(loc token.SourceLoc) ast.Expr {
	p.expect(token.LBRACE)
	se := &ast.SelectExpr{Loc: loc}
	for p.cur().Type != token.RBRACE && p.cur().Type != token.EOF {
		cur := p.cur()
		if cur.Type == token.IDENT && cur.Lexeme == "case" {
			p.advance() // consume "case"
			c := ast.SelectCase{}
			if p.cur().Type == token.IDENT && p.peek().Type == token.EQUALS {
				c.Bind = p.advance().Lexeme
				p.advance() // =
			}
			c.Recv = p.parseExpr(0)
			p.expect(token.ARROW)
			c.Body = p.parseExpr(0)
			se.Cases = append(se.Cases, c)
		} else if cur.Type == token.IDENT && cur.Lexeme == "default" {
			p.advance() // consume "default"
			p.expect(token.ARROW)
			se.Default = p.parseExpr(0)
			break
		} else {
			break
		}
	}
	p.expect(token.RBRACE)
	return se
}

// parseCompExpr parses `builder { ops }` where builder was already consumed.
func (p *Parser) parseCompExpr(loc token.SourceLoc, builder string) ast.Expr {
	p.expect(token.LBRACE)
	ce := &ast.CompExpr{Builder: builder, Loc: loc}

	for p.cur().Type != token.RBRACE && p.cur().Type != token.EOF {
		op := p.parseCompOp()
		if op != nil {
			ce.Ops = append(ce.Ops, op)
		}
		// Optional semicolon or newline separator
		p.match(token.SEMI)
	}
	p.expect(token.RBRACE)
	return ce
}

// parseCompOp parses one operation inside a computation expression.
func (p *Parser) parseCompOp() ast.CompOp {
	switch {
	case p.cur().Type == token.LET && p.peek().Type == token.BANG:
		// let! pattern = expr
		p.advance() // let
		p.advance() // !
		pat := p.parsePattern()
		p.expect(token.EQUALS)
		expr := p.parseExpr(0)
		return &ast.LetBangOp{Pattern: pat, Expr: expr}

	case p.cur().Type == token.LET:
		// let pattern = expr
		p.advance()
		pat := p.parsePattern()
		p.expect(token.EQUALS)
		expr := p.parseExpr(0)
		return &ast.LetOp{Pattern: pat, Expr: expr}

	case p.cur().Type == token.IDENT && p.cur().Lexeme == "do" && p.peek().Type == token.BANG:
		// do! expr
		p.advance() // do
		p.advance() // !
		expr := p.parseExpr(0)
		return &ast.DoBangOp{Expr: expr}

	case p.cur().Type == token.IDENT && p.cur().Lexeme == "return" && p.peek().Type == token.BANG:
		// return! expr
		p.advance() // return
		p.advance() // !
		expr := p.parseExpr(0)
		return &ast.ReturnBangOp{Expr: expr}

	case p.cur().Type == token.IDENT && p.cur().Lexeme == "return":
		// return expr
		p.advance()
		expr := p.parseExpr(0)
		return &ast.ReturnOp{Expr: expr}

	default:
		// Body expression
		expr := p.parseExpr(0)
		return &ast.BodyOp{Expr: expr}
	}
}
