package parser

import (
	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/token"
)

// parseModuleDecl parses nested modules, functors, and module types.
// Called when cur is MODULE.
func (p *Parser) parseModuleDecl() ast.TopDecl {
	p.expect(token.MODULE)

	// module type S = sig ... end
	if p.match(token.TYPE) {
		return p.parseModuleTypeDecl()
	}

	rec := p.match(token.REC)
	d := p.parseNamedModuleDecl(rec)
	if d == nil {
		return nil
	}
	for rec && p.match(token.AND) {
		peer := p.parseNamedModuleDecl(true)
		if peer != nil {
			d.RecDecls = append(d.RecDecls, peer)
		}
	}
	return d
}

func (p *Parser) parseNamedModuleDecl(rec bool) *ast.NestedModuleDecl {
	name := p.parseQualifiedName()
	if name == "" {
		p.errorf("expected module name")
		return nil
	}

	d := &ast.NestedModuleDecl{Name: name, Rec: rec}

	// Functor: module F (X : S) = ...
	if p.match(token.LPAREN) {
		argTok := p.cur()
		if argTok.Type == token.IDENT || argTok.Type == token.CONSTRUCTOR {
			p.advance()
			d.FunctorArg = argTok.Lexeme
		}
		p.expect(token.COLON)
		sig := p.parseQualifiedName()
		d.FunctorSig = sig
		p.expect(token.RPAREN)
	}

	if p.match(token.COLON) {
		d.SealSig = p.parseSignatureRef()
	}
	p.expect(token.EQUALS)

	switch {
	case p.cur().Type == token.STRUCT:
		d.Decls = p.parseStructBody()
	case p.cur().Type == token.FUNCTOR:
		p.advance()
		p.expect(token.LPAREN)
		arg := p.cur()
		if arg.Type == token.IDENT || arg.Type == token.CONSTRUCTOR {
			p.advance()
			d.FunctorArg = arg.Lexeme
		}
		p.expect(token.COLON)
		d.FunctorSig = p.parseSignatureRef()
		p.expect(token.RPAREN)
		p.expect(token.ARROW)
		if p.cur().Type == token.STRUCT {
			d.Decls = p.parseStructBody()
		} else {
			p.errorf("expected struct after functor argument")
		}
	case p.cur().Type == token.CONSTRUCTOR || p.cur().Type == token.IDENT:
		// Functor application: F(M) or just alias M
		funcName := p.parseQualifiedName()
		if p.match(token.LPAREN) {
			arg := p.parseQualifiedName()
			p.expect(token.RPAREN)
			d.IsApp = true
			d.AppFunc = funcName
			d.AppArg = arg
		} else {
			// module Alias = Other — treat as include of Other's path
			d.IsApp = true
			d.AppFunc = ""
			d.AppArg = funcName
		}
	default:
		p.errorf("expected struct or module expression after '=', got %s", p.cur().Type)
	}
	return d
}

func (p *Parser) parseModuleTypeDecl() *ast.ModuleTypeDecl {
	d := &ast.ModuleTypeDecl{}
	d.Name = p.parseQualifiedName()
	p.expect(token.EQUALS)
	if p.match(token.MODULE) {
		p.expect(token.TYPE)
		p.expect(token.OF)
		d.OfModule = p.parseQualifiedName()
		return d
	}
	p.expect(token.SIG)
	for p.cur().Type != token.END && p.cur().Type != token.EOF {
		item := p.parseSigItem()
		if item.Name != "" || item.Kind != "" {
			d.Items = append(d.Items, item)
		} else {
			break
		}
	}
	p.expect(token.END)
	d.WithConstraints = p.parseWithConstraints()
	return d
}

func (p *Parser) parseSignatureRef() string {
	name := p.parseQualifiedName()
	// Constraints are retained in the AST but don't change this signature's
	// value surface in the pragmatic checker.
	_ = p.parseWithConstraints()
	return name
}

func (p *Parser) parseWithConstraints() []ast.SigConstraint {
	var constraints []ast.SigConstraint
	for p.match(token.WITH) {
		c := ast.SigConstraint{}
		switch p.cur().Type {
		case token.TYPE:
			c.Kind = "type"
		case token.MODULE:
			c.Kind = "module"
		default:
			p.errorf("expected type or module after with")
			return constraints
		}
		p.advance()
		c.Name = p.parseQualifiedName()
		if p.match(token.COLONEQ) {
			c.Destructive = true
		} else {
			p.expect(token.EQUALS)
		}
		c.Manifest = p.parseQualifiedName()
		constraints = append(constraints, c)
	}
	return constraints
}

func (p *Parser) parseSigItem() ast.SigItem {
	item := ast.SigItem{}
	switch p.cur().Type {
	case token.VAL:
		p.advance()
		item.Kind = "val"
		tok := p.cur()
		if tok.Type == token.IDENT || tok.Type == token.CONSTRUCTOR {
			p.advance()
			item.Name = tok.Lexeme
		}
		p.expect(token.COLON)
		item.Type = p.parseType()
	case token.TYPE:
		p.advance()
		item.Kind = "type"
		tok := p.cur()
		if tok.Type == token.IDENT || tok.Type == token.CONSTRUCTOR || tok.Type == token.UNDERSCORE {
			p.advance()
			item.Name = tok.Lexeme
		}
		// Optional `= ...` abstract or manifest — skip lightly
		if p.match(token.EQUALS) {
			_ = p.parseType()
		}
	default:
		p.errorf("expected val or type in sig, got %s", p.cur().Type)
		p.advance()
	}
	return item
}

func (p *Parser) parseStructBody() []ast.TopDecl {
	p.expect(token.STRUCT)
	var decls []ast.TopDecl
	for p.cur().Type != token.END && p.cur().Type != token.EOF {
		d := p.parseStructItem()
		if d != nil {
			decls = append(decls, d)
		} else if p.cur().Type != token.END {
			p.advance()
		}
	}
	p.expect(token.END)
	return decls
}

func (p *Parser) parseStructItem() ast.TopDecl {
	switch p.cur().Type {
	case token.LET:
		return p.parseLetDecl(false)
	case token.TYPE:
		return p.parseTypeDecl(false)
	case token.EXCEPTION:
		return p.parseExceptionDecl()
	case token.EFFECT:
		return p.parseEffectDecl()
	case token.MODULE:
		return p.parseModuleDecl()
	case token.OPEN:
		return p.parseOpenDecl()
	case token.INCLUDE:
		return p.parseIncludeDecl()
	case token.CLASS:
		return p.parseClassDecl()
	case token.PRIVATE:
		p.advance()
		switch p.cur().Type {
		case token.LET:
			return p.parseLetDecl(true)
		case token.TYPE:
			return p.parseTypeDecl(true)
		default:
			p.errorf("expected let or type after private")
			return nil
		}
	default:
		p.errorf("unexpected token %s in struct", p.cur().Type)
		return nil
	}
}

func (p *Parser) parseOpenDecl() *ast.OpenModuleDecl {
	p.expect(token.OPEN)
	force := p.match(token.BANG)
	path := p.parseQualifiedName()
	return &ast.OpenModuleDecl{Path: path, Force: force}
}

func (p *Parser) parseIncludeDecl() *ast.IncludeDecl {
	p.expect(token.INCLUDE)
	path := p.parseQualifiedName()
	return &ast.IncludeDecl{Path: path}
}

func (p *Parser) parseClassDecl() *ast.ClassDecl {
	p.expect(token.CLASS)
	d := &ast.ClassDecl{}
	d.TypeOnly = p.match(token.TYPE)
	d.Virtual = p.match(token.VIRTUAL)
	tok := p.cur()
	if tok.Type == token.IDENT || tok.Type == token.CONSTRUCTOR {
		p.advance()
		d.Name = tok.Lexeme
	} else {
		p.errorf("expected class name, got %s", tok.Type)
	}
	p.expect(token.EQUALS)
	obj := p.parseObjectBody(d.TypeOnly)
	d.Self = obj.Self
	d.Fields = obj.Fields
	d.Methods = obj.Methods
	d.Inherits = obj.Inherits
	d.Initializers = obj.Initializers
	d.Constraints = obj.Constraints
	return d
}

func (p *Parser) parseObjectBody(typeOnly bool) *ast.ObjectExpr {
	p.expect(token.OBJECT)
	obj := &ast.ObjectExpr{}
	if p.match(token.LPAREN) {
		tok := p.cur()
		if tok.Type == token.IDENT {
			p.advance()
			obj.Self = tok.Lexeme
		}
		p.expect(token.RPAREN)
	}
	for p.cur().Type != token.END && p.cur().Type != token.EOF {
		switch p.cur().Type {
		case token.VAL:
			p.advance()
			f := ast.ClassField{}
			if p.match(token.MUTABLE) {
				f.Mutable = true
			}
			nameTok := p.cur()
			if nameTok.Type == token.IDENT {
				p.advance()
				f.Name = nameTok.Lexeme
			}
			p.expect(token.EQUALS)
			f.Value = p.parseExpr(0)
			obj.Fields = append(obj.Fields, f)
		case token.METHOD:
			p.advance()
			m := ast.ClassMethod{}
			for p.cur().Type == token.PRIVATE || p.cur().Type == token.VIRTUAL {
				if p.match(token.PRIVATE) {
					m.Private = true
				} else if p.match(token.VIRTUAL) {
					m.Virtual = true
				}
			}
			nameTok := p.cur()
			if nameTok.Type == token.IDENT || nameTok.Type == token.CONSTRUCTOR {
				p.advance()
				m.Name = nameTok.Lexeme
			}
			if m.Virtual {
				p.expect(token.COLON)
				m.Type = p.parseType()
				obj.Methods = append(obj.Methods, m)
				continue
			}
			m.Params = p.parseParams()
			if p.match(token.COLON) {
				m.Type = p.parseType()
			}
			if typeOnly {
				if m.Type == nil {
					p.errorf("class type method %s requires a type", m.Name)
				}
				obj.Methods = append(obj.Methods, m)
				continue
			}
			p.expect(token.EQUALS)
			m.Body = p.parseExpr(0)
			obj.Methods = append(obj.Methods, m)
		case token.INHERIT:
			p.advance()
			p.match(token.NEW)
			name := p.parseQualifiedName()
			if name == "" {
				p.errorf("expected class name after inherit")
			} else {
				obj.Inherits = append(obj.Inherits, name)
			}
		case token.INITIALIZER:
			p.advance()
			obj.Initializers = append(obj.Initializers, p.parseExpr(0))
		case token.CONSTRAINT:
			p.advance()
			left := p.parseType()
			p.expect(token.EQUALS)
			right := p.parseType()
			obj.Constraints = append(obj.Constraints, ast.ClassConstraint{Left: left, Right: right})
		default:
			p.errorf("expected val, method, inherit, initializer, or constraint in object, got %s", p.cur().Type)
			p.advance()
		}
	}
	p.expect(token.END)
	return obj
}

func (p *Parser) parseObjectExpr() ast.Expr {
	loc := p.cur().Loc
	obj := p.parseObjectBody(false)
	obj.Loc = loc
	return obj
}

func (p *Parser) parseNewExpr() ast.Expr {
	loc := p.cur().Loc
	p.expect(token.NEW)
	name := p.parseQualifiedName()
	return &ast.NewExpr{Class: name, Loc: loc}
}

func (p *Parser) parseLetModuleExpr() ast.Expr {
	loc := p.cur().Loc
	p.expect(token.LET)
	p.expect(token.MODULE)
	name := p.parseQualifiedName()
	p.expect(token.EQUALS)
	var decls []ast.TopDecl
	if p.cur().Type == token.STRUCT {
		decls = p.parseStructBody()
	} else {
		p.errorf("expected struct after let module")
	}
	p.match(token.IN)
	body := p.parseExpr(0)
	return &ast.LetModuleExpr{Name: name, Decls: decls, Body: body, Loc: loc}
}

func (p *Parser) parseLabelledArg() ast.Expr {
	loc := p.cur().Loc
	p.expect(token.TILDE)
	tok := p.cur()
	if tok.Type != token.IDENT && tok.Type != token.CONSTRUCTOR {
		p.errorf("expected label after ~, got %s", tok.Type)
		return &ast.LabelledArgExpr{Loc: loc}
	}
	p.advance()
	label := tok.Lexeme
	if p.match(token.COLON) {
		return &ast.LabelledArgExpr{Label: label, Value: p.parsePrefix(), Loc: loc}
	}
	return &ast.LabelledArgExpr{Label: label, Value: nil, Loc: loc}
}

func (p *Parser) parseOptionalLabelledArg() ast.Expr {
	loc := p.cur().Loc
	p.expect(token.QUESTION)
	tok := p.cur()
	if tok.Type != token.IDENT && tok.Type != token.CONSTRUCTOR {
		p.errorf("expected label after ?, got %s", tok.Type)
		return &ast.LabelledArgExpr{Loc: loc, Optional: true}
	}
	p.advance()
	arg := &ast.LabelledArgExpr{Label: tok.Lexeme, Optional: true, Loc: loc}
	if p.match(token.COLON) {
		arg.Value = p.parsePrefix()
	}
	return arg
}
