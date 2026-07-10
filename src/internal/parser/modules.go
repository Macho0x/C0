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

	name := p.parseQualifiedName()
	if name == "" {
		p.errorf("expected module name")
		return nil
	}

	d := &ast.NestedModuleDecl{Name: name}

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

	p.expect(token.EQUALS)

	switch {
	case p.cur().Type == token.STRUCT:
		d.Decls = p.parseStructBody()
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
	return d
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
	path := p.parseQualifiedName()
	return &ast.OpenModuleDecl{Path: path}
}

func (p *Parser) parseIncludeDecl() *ast.IncludeDecl {
	p.expect(token.INCLUDE)
	path := p.parseQualifiedName()
	return &ast.IncludeDecl{Path: path}
}

func (p *Parser) parseClassDecl() *ast.ClassDecl {
	p.expect(token.CLASS)
	d := &ast.ClassDecl{}
	tok := p.cur()
	if tok.Type == token.IDENT || tok.Type == token.CONSTRUCTOR {
		p.advance()
		d.Name = tok.Lexeme
	} else {
		p.errorf("expected class name, got %s", tok.Type)
	}
	p.expect(token.EQUALS)
	self, fields, methods := p.parseObjectBody()
	d.Self = self
	d.Fields = fields
	d.Methods = methods
	return d
}

func (p *Parser) parseObjectBody() (self string, fields []ast.ClassField, methods []ast.ClassMethod) {
	p.expect(token.OBJECT)
	if p.match(token.LPAREN) {
		tok := p.cur()
		if tok.Type == token.IDENT {
			p.advance()
			self = tok.Lexeme
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
			fields = append(fields, f)
		case token.METHOD:
			p.advance()
			m := ast.ClassMethod{}
			nameTok := p.cur()
			if nameTok.Type == token.IDENT || nameTok.Type == token.CONSTRUCTOR {
				p.advance()
				m.Name = nameTok.Lexeme
			}
			m.Params = p.parseParams()
			p.expect(token.EQUALS)
			m.Body = p.parseExpr(0)
			methods = append(methods, m)
		default:
			p.errorf("expected val or method in object, got %s", p.cur().Type)
			p.advance()
		}
	}
	p.expect(token.END)
	return self, fields, methods
}

func (p *Parser) parseObjectExpr() ast.Expr {
	loc := p.cur().Loc
	self, fields, methods := p.parseObjectBody()
	return &ast.ObjectExpr{Self: self, Fields: fields, Methods: methods, Loc: loc}
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
