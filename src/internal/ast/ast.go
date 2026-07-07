// Package ast defines the Abstract Syntax Tree types for C0.
package ast

import (
	"c0.dev/compiler/internal/token"
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Top-level nodes
// ---------------------------------------------------------------------------

// Module represents a complete C0 source file.
type Module struct {
	Name   string        // module path, e.g. "Trading.OrderBook"
	Opens  []OpenStmt    // open directives
	Decls  []TopDecl     // top-level declarations
}

// OpenStmt is an `open Path` statement.
type OpenStmt struct {
	Path string // dot-separated module path
}

// ---------------------------------------------------------------------------
// Top-level declarations
// ---------------------------------------------------------------------------

// TopDecl is the interface for module-level declarations.
type TopDecl interface {
	topDeclNode()
}

// LetDecl is a value binding at the top level.
type LetDecl struct {
	Rec           bool
	Mutable       bool
	ActivePattern bool        // true for `let (|Name|_|) …`
	Bindings      []LetBinding // multiple when `and` is used
}

func (*LetDecl) topDeclNode() {}

// TypeDecl is a type declaration at the top level.
type TypeDecl struct {
	Name       string
	TypeParams []string // e.g. ["'a"]
	Kind       TypeKind
	Quantity   int // 0 = unrestricted (default), 1 = linear
}

func (*TypeDecl) topDeclNode() {}

// ExternDecl is an `extern` block (parsed but not elaborated).
type ExternDecl struct {
	Lang     string      // e.g. "go"
	Path     string      // e.g. "github.com/example/lib"
	Vals     []ExternVal
	GoBlocks []string    // raw Go source code from go { ... } blocks (inline Go extern)
}

func (*ExternDecl) topDeclNode() {}

// ExternVal is a single `val` inside an extern block.
type ExternVal struct {
	Name string
	Type Type
}

// ---------------------------------------------------------------------------
// Bindings & parameters
// ---------------------------------------------------------------------------

// LetBinding represents one arm of a (possibly mutually-recursive) let.
type LetBinding struct {
	Name       string
	Params     []Param
	RetType    Type           // nil if omitted
	RetEffects *EffectRowType // nil if no `with` clause
	Body       Expr
}

// Param is a function parameter.
type Param struct {
	Name string
	Type Type // nil if unannotated (e.g. `fun x ->`)
}

// ---------------------------------------------------------------------------
// TypeKind: record vs ADT vs alias
// ---------------------------------------------------------------------------

type TypeKind interface {
	typeKindNode()
}

// RecordTypeKind is `type T = { field: Type; ... }`.
type RecordTypeKind struct {
	Fields []FieldType
}

func (*RecordTypeKind) typeKindNode() {}

// ADTTypeKind is `type T = | Case1 of ... | Case2 | ...`.
type ADTTypeKind struct {
	Cases []ADTCase
}

func (*ADTTypeKind) typeKindNode() {}

// AliasTypeKind is `type T = SomeType` (a type alias).
type AliasTypeKind struct {
	Alias Type
}

func (*AliasTypeKind) typeKindNode() {}

// OpaqueTypeKind is `type T : 1` — an opaque linear type with no body.
type OpaqueTypeKind struct{}

func (*OpaqueTypeKind) typeKindNode() {}

// ADTCase is one constructor alternative in an ADT.
type ADTCase struct {
	Name string
	Arg  Type // nil if no payload (e.g. `| Point`)
}

// FieldType is a record field declaration: `name : Type`.
type FieldType struct {
	Name string
	Type Type
}

// ---------------------------------------------------------------------------
// Type expressions
// ---------------------------------------------------------------------------

// Type is an interface for type expressions.
type Type interface {
	typeNode()
}

// TIdent is a named type: `int`, `string`, `MyModule.t`.
type TIdent struct {
	Name string // may contain dots for qualified names
}

func (*TIdent) typeNode() {}

// TApp is type application: `order list`, `(int, string) result`.
// Func is the "outer" type, Arg is the parameter.
type TApp struct {
	Func Type
	Arg  Type
}

func (*TApp) typeNode() {}

// TFun is a function type: `A -> B`.
type TFun struct {
	From    Type
	To      Type
	Effects *EffectRowType // nil if no `with` clause
}

func (*TFun) typeNode() {}

// EffectRowType is an effect row annotation: `with { io; log }` or `with { e | .. }`.
type EffectRowType struct {
	Effects []string // effect names, e.g. "io", "log"
	Open    bool     // `| ..` present
	Rest    string   // row variable name (before `| ..`), or ""
}

func (*EffectRowType) typeNode() {}

// TTuple is a tuple type: `(A, B, C)`.
type TTuple struct {
	Elems []Type
}

func (*TTuple) typeNode() {}

// TRecord is a record type: `{ id: int; name: string }`.
// If Open is true, it's a row type: `{ name: string | .. }`.
type TRecord struct {
	Fields []FieldType
	Open   bool // true for row-polymorphic `{ ... | .. }`
}

func (*TRecord) typeNode() {}

// TVar is a type variable: `'a`.
type TVar struct {
	Name string // includes the leading apostrophe, e.g. "'a"
}

func (*TVar) typeNode() {}

// TChan is a channel type: `int chan`.
type TChan struct {
	Elem Type
}

func (*TChan) typeNode() {}

// RefinementType wraps a type with a where clause.
// `int where x > 0` → RefinementType{Inner: int, Pred: x > 0}
type RefinementType struct {
	Inner Type // the base type
	Pred  Expr // the predicate expression
}

func (*RefinementType) typeNode() {}

// ---------------------------------------------------------------------------
// Expressions
// ---------------------------------------------------------------------------

// Expr is the interface for all expression nodes.
type Expr interface {
	exprNode()
}

// LitExpr is a literal: `42`, `3.14`, `"hello"`, `true`, `false`, `()`.
type LitExpr struct {
	Value any             // int64, float64, string, bool, or nil for unit
	Kind  token.TokenType // INT, FLOAT, STRING, TRUE, FALSE, UNIT
	Loc   token.SourceLoc // source location
}

func (*LitExpr) exprNode() {}

// IdentExpr is an identifier reference: `x`, `Console.print_line`.
type IdentExpr struct {
	Name string
	Loc  token.SourceLoc // source location
}

func (*IdentExpr) exprNode() {}

// ConstructorExpr is a constructor (capitalised) used in expression position.
// e.g. `None`, `Some 42`.
type ConstructorExpr struct {
	Name string
	Arg  Expr             // nil when no argument
	Loc  token.SourceLoc  // source location
}

func (*ConstructorExpr) exprNode() {}

// AppExpr is function application: `f x y`.
type AppExpr struct {
	Func Expr
	Arg  Expr
	Loc  token.SourceLoc // source location
}

func (*AppExpr) exprNode() {}

// IfExpr is `if cond then trueBranch else falseBranch`.
type IfExpr struct {
	Cond       Expr
	ThenBranch Expr
	ElseBranch Expr
	Loc        token.SourceLoc // source location
}

func (*IfExpr) exprNode() {}

// MatchExpr is `match scrutinee with | pat -> body | ...`.
type MatchExpr struct {
	Scrutinee Expr
	Arms      []MatchArm
	Loc       token.SourceLoc // source location
}

func (*MatchExpr) exprNode() {}

// MatchArm is one branch of a match expression.
type MatchArm struct {
	Pattern Pattern
	Guard   Expr // nil if no `when`
	Body    Expr
}

// LetInExpr is `let binding in body`.
type LetInExpr struct {
	Mutable   bool            // true for `let mutable … in …`
	Bindings  []LetBinding
	Body      Expr
	Loc       token.SourceLoc // source location
}

func (*LetInExpr) exprNode() {}

// FunExpr is `fun params -> body`.
type FunExpr struct {
	Params []Param
	Body   Expr
	Loc    token.SourceLoc // source location
}

func (*FunExpr) exprNode() {}

// GuardExpr is `guard pat = expr else expr` (single binding)
// or `guard pat1 = expr1 and pat2 = expr2 else expr` (multiple bindings).
type GuardExpr struct {
	Bindings []GuardBinding // at least one
	Else_    Expr
	Loc      token.SourceLoc // source location
}

// GuardBinding is one arm of a guard expression: `pattern = expr`.
type GuardBinding struct {
	Pattern Pattern
	Expr    Expr
}

func (*GuardExpr) exprNode() {}

// IsExpr is `expr is pattern` (match macro).
type IsExpr struct {
	Left    Expr
	Pattern Pattern
	Loc     token.SourceLoc // source location
}

func (*IsExpr) exprNode() {}

// AsMatchExpr is `expr as pat -> body else elseBody` (match macro).
type AsMatchExpr struct {
	Left     Expr
	Pattern  Pattern
	Body     Expr
	ElseBody Expr
	Loc      token.SourceLoc // source location
}

func (*AsMatchExpr) exprNode() {}

// BinaryExpr is a binary operator expression: `a + b`, `x :: xs`, etc.
type BinaryExpr struct {
	Left  Expr
	Op    token.TokenType
	Right Expr
	Loc   token.SourceLoc // source location
}

func (*BinaryExpr) exprNode() {}

// PipeExpr is a pipeline: `left |> right`.
type PipeExpr struct {
	Left  Expr
	Right Expr
	Loc   token.SourceLoc // source location
}

func (*PipeExpr) exprNode() {}

// QuestionExpr is the error-propagation operator: `expr ?` or `expr ? arg`.
type QuestionExpr struct {
	Left Expr
	Arg  Expr            // nil for bare `?`
	Loc  token.SourceLoc // source location
}

func (*QuestionExpr) exprNode() {}

// RecordExpr is a record literal: `{ x = 1; y = 2 }` or `{ x; y }` (punning).
type RecordExpr struct {
	Fields []RecordField
	Loc    token.SourceLoc // source location
}

func (*RecordExpr) exprNode() {}

// RecordField is one field in a record literal.
type RecordField struct {
	Name  string
	Value Expr // nil for punning
}

// RecordUpdateExpr is `{ expr with field = value; ... }`.
type RecordUpdateExpr struct {
	Base   Expr
	Fields []RecordField // all have Values set
	Loc    token.SourceLoc // source location
}

func (*RecordUpdateExpr) exprNode() {}

// FieldAccessExpr is `expr.field`.
type FieldAccessExpr struct {
	Left  Expr
	Field string
	Loc   token.SourceLoc // source location
}

func (*FieldAccessExpr) exprNode() {}

// TupleExpr is a tuple literal: `(a, b, c)`.
type TupleExpr struct {
	Elems []Expr
	Loc   token.SourceLoc // source location
}

func (*TupleExpr) exprNode() {}

// ListExpr is a list literal: `[a; b; c]` or `[]`.
type ListExpr struct {
	Elems []Expr           // nil for `[]`
	Loc   token.SourceLoc  // source location
}

func (*ListExpr) exprNode() {}

// ParenExpr is a parenthesised expression: `(expr)`.
type ParenExpr struct {
	Inner Expr
	Loc   token.SourceLoc // source location
}

func (*ParenExpr) exprNode() {}

// CompExpr is a computation expression: `builder { ops }`.
type CompExpr struct {
	Builder string  // e.g. "result", "async"
	Ops     []CompOp
	Loc     token.SourceLoc // source location
}

func (*CompExpr) exprNode() {}

// CompOp is one operation inside a computation expression.
type CompOp interface {
	compOpNode()
}

// LetBangOp is `let! pattern = expr` inside a CE.
type LetBangOp struct {
	Pattern Pattern
	Expr    Expr
}

func (*LetBangOp) compOpNode() {}

// DoBangOp is `do! expr` inside a CE.
type DoBangOp struct {
	Expr Expr
}

func (*DoBangOp) compOpNode() {}

// LetOp is `let pattern = expr` inside a CE.
type LetOp struct {
	Pattern Pattern
	Expr    Expr
}

func (*LetOp) compOpNode() {}

// ReturnOp is `return expr` inside a CE.
type ReturnOp struct {
	Expr Expr
}

func (*ReturnOp) compOpNode() {}

// ReturnBangOp is `return! expr` inside a CE.
type ReturnBangOp struct {
	Expr Expr
}

func (*ReturnBangOp) compOpNode() {}

// BodyOp is the final expression in a CE (no keyword).
type BodyOp struct {
	Expr Expr
}

func (*BodyOp) compOpNode() {}

// GoExpr is a `go expr` expression.
type GoExpr struct {
	Expr Expr
	Loc  token.SourceLoc // source location
}

func (*GoExpr) exprNode() {}

// SelectExpr is a `select { case x = expr -> body ... }` expression.
type SelectExpr struct {
	Cases   []SelectCase
	Default Expr            // nil if no default
	Loc     token.SourceLoc // source location
}

func (*SelectExpr) exprNode() {}

// UsingExpr is a `using pat = expr in body` expression.
type UsingExpr struct {
	Pattern Pattern
	Expr    Expr
	Body    Expr
	Loc     token.SourceLoc // source location
}

func (*UsingExpr) exprNode() {}

// RegionExpr is a desugared `region { ops }` computation expression for
// scoped linear resource management. Each let! binding emits a defer
// Close(varName) in codegen, and the linear checker auto-discharges
// any live linear variables at the region exit.
type RegionExpr struct {
	Ops []CompOp
	Loc token.SourceLoc // source location
}

func (*RegionExpr) exprNode() {}

// SelectCase is one case in a select expression.
type SelectCase struct {
	Bind string // variable binding for receive case
	Recv Expr   // the expression to receive from (of channel type)
	Body Expr
}

// ---------------------------------------------------------------------------
// Patterns
// ---------------------------------------------------------------------------

// Pattern is the interface for match patterns.
type Pattern interface {
	patternNode()
}

// WildcardPattern is `_`.
type WildcardPattern struct{}

func (*WildcardPattern) patternNode() {}

// IdentPattern is a variable pattern: `x`, captures a value.
type IdentPattern struct {
	Name string
}

func (*IdentPattern) patternNode() {}

// LitPattern matches a literal: `42`, `"hello"`, `true`.
type LitPattern struct {
	Value any
	Kind  token.TokenType
}

func (*LitPattern) patternNode() {}

// ConstructorPattern matches a constructor: `None`, `Some x`, `Circle { radius }`.
type ConstructorPattern struct {
	Name string
	Arg  Pattern // nil when no payload
}

func (*ConstructorPattern) patternNode() {}

// RecordPattern matches a record: `{ x; y }` or `{ x = pat }`.
type RecordPattern struct {
	Fields []RecordPatField
}

func (*RecordPattern) patternNode() {}

// RecordPatField is a field within a record pattern.
type RecordPatField struct {
	Name    string
	Pattern Pattern // nil for punning (binds to same name)
}

// TuplePattern matches a tuple: `(a, b)`.
type TuplePattern struct {
	Elems []Pattern
}

func (*TuplePattern) patternNode() {}

// ListPattern matches a list: `[]` or `[a; b]`.
type ListPattern struct {
	Elems []Pattern // nil for `[]`
}

func (*ListPattern) patternNode() {}

// ConsPattern matches `head :: tail`.
type ConsPattern struct {
	Head Pattern
	Tail Pattern
}

func (*ConsPattern) patternNode() {}

// AliasPattern is `pat as name`.
type AliasPattern struct {
	Pattern Pattern
	Name    string
}

func (*AliasPattern) patternNode() {}

// ---------------------------------------------------------------------------
// Location container (optional — used by nodes that carry source info)
// ---------------------------------------------------------------------------

// Located wraps a node with source location.
type Located[T any] struct {
	Node T
	Loc  token.SourceLoc
}

// ---------------------------------------------------------------------------
// Pretty-printing helpers (for debugging / parser tests)
// ---------------------------------------------------------------------------

// typeString converts a Type to a human-readable string.
func typeString(t Type) string {
	switch t := t.(type) {
	case *TIdent:
		return t.Name
	case *TApp:
		return fmt.Sprintf("%s(%s)", typeString(t.Func), typeString(t.Arg))
	case *TFun:
		s := typeString(t.From) + " -> " + typeString(t.To)
		if t.Effects != nil {
			effs := strings.Join(t.Effects.Effects, "; ")
			if t.Effects.Open {
				if t.Effects.Rest != "" {
					effs += " | " + t.Effects.Rest
				} else {
					effs += " | .."
				}
			}
			if effs != "" {
				s += " with {" + effs + "}"
			}
		}
		return s
	case *TTuple:
		parts := make([]string, len(t.Elems))
		for i, e := range t.Elems {
			parts[i] = typeString(e)
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case *TRecord:
		parts := make([]string, len(t.Fields))
		for i, f := range t.Fields {
			parts[i] = f.Name + ": " + typeString(f.Type)
		}
		s := "{ " + strings.Join(parts, "; ") + " "
		if t.Open {
			s += "| .. "
		}
		return s + "}"
	case *TChan:
		return typeString(t.Elem) + " chan"
	case *RefinementType:
		return typeString(t.Inner) + " where " + ExprString(t.Pred)
	case *TVar:
		return t.Name
	default:
		return "<unknown type>"
	}
}

// ExprString converts an Expr to a human-readable string.
func ExprString(e Expr) string {
	switch e := e.(type) {
	case *LitExpr:
		return fmt.Sprintf("%v", e.Value)
	case *IdentExpr:
		return e.Name
	case *ConstructorExpr:
		if e.Arg != nil {
			return e.Name + "(" + ExprString(e.Arg) + ")"
		}
		return e.Name
	case *AppExpr:
		return "App(" + ExprString(e.Func) + ", " + ExprString(e.Arg) + ")"
	case *IfExpr:
		return fmt.Sprintf("If(%s, %s, %s)", ExprString(e.Cond), ExprString(e.ThenBranch), ExprString(e.ElseBranch))
	case *MatchExpr:
		arms := make([]string, len(e.Arms))
		for i, a := range e.Arms {
			arms[i] = patternString(a.Pattern) + " -> " + ExprString(a.Body)
		}
		return "Match(" + ExprString(e.Scrutinee) + ", [" + strings.Join(arms, " | ") + "])"
	case *LetInExpr:
		bs := make([]string, len(e.Bindings))
		for i, b := range e.Bindings {
			bs[i] = b.Name + " = " + ExprString(b.Body)
		}
		return "Let([" + strings.Join(bs, "; ") + "], " + ExprString(e.Body) + ")"
	case *FunExpr:
		ps := make([]string, len(e.Params))
		for i, p := range e.Params {
			ps[i] = p.Name
		}
		return "Fun([" + strings.Join(ps, ", ") + "], " + ExprString(e.Body) + ")"
	case *GuardExpr:
		bs := make([]string, len(e.Bindings))
		for i, b := range e.Bindings {
			bs[i] = patternString(b.Pattern) + " = " + ExprString(b.Expr)
		}
		return "Guard(" + strings.Join(bs, " and ") + " else " + ExprString(e.Else_) + ")"
	case *IsExpr:
		return "Is(" + ExprString(e.Left) + ", " + patternString(e.Pattern) + ")"
	case *AsMatchExpr:
		return "AsMatch(" + ExprString(e.Left) + ", " + patternString(e.Pattern) + " -> " + ExprString(e.Body) + " else " + ExprString(e.ElseBody) + ")"
	case *BinaryExpr:
		return fmt.Sprintf("Bin(%s, %s, %s)", ExprString(e.Left), e.Op, ExprString(e.Right))
	case *PipeExpr:
		return "Pipe(" + ExprString(e.Left) + " |> " + ExprString(e.Right) + ")"
	case *QuestionExpr:
		if e.Arg != nil {
			return "Q(" + ExprString(e.Left) + " ? " + ExprString(e.Arg) + ")"
		}
		return "Q(" + ExprString(e.Left) + " ?)"
	case *RecordExpr:
		parts := make([]string, len(e.Fields))
		for i, f := range e.Fields {
			if f.Value != nil {
				parts[i] = f.Name + " = " + ExprString(f.Value)
			} else {
				parts[i] = f.Name
			}
		}
		return "{" + strings.Join(parts, "; ") + "}"
	case *RecordUpdateExpr:
		parts := make([]string, len(e.Fields))
		for i, f := range e.Fields {
			parts[i] = f.Name + " = " + ExprString(f.Value)
		}
		return "{" + ExprString(e.Base) + " with " + strings.Join(parts, "; ") + "}"
	case *FieldAccessExpr:
		return ExprString(e.Left) + "." + e.Field
	case *TupleExpr:
		parts := make([]string, len(e.Elems))
		for i, el := range e.Elems {
			parts[i] = ExprString(el)
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case *ListExpr:
		parts := make([]string, len(e.Elems))
		for i, el := range e.Elems {
			parts[i] = ExprString(el)
		}
		return "[" + strings.Join(parts, "; ") + "]"
	case *ParenExpr:
		return "(" + ExprString(e.Inner) + ")"
	case *CompExpr:
		ops := make([]string, len(e.Ops))
		for i, op := range e.Ops {
			switch o := op.(type) {
			case *LetBangOp:
				ops[i] = fmt.Sprintf("let! %s = %s", patternString(o.Pattern), ExprString(o.Expr))
			case *DoBangOp:
				ops[i] = fmt.Sprintf("do! %s", ExprString(o.Expr))
			case *LetOp:
				ops[i] = fmt.Sprintf("let %s = %s", patternString(o.Pattern), ExprString(o.Expr))
			case *ReturnOp:
				ops[i] = fmt.Sprintf("return %s", ExprString(o.Expr))
			case *ReturnBangOp:
				ops[i] = fmt.Sprintf("return! %s", ExprString(o.Expr))
			case *BodyOp:
				ops[i] = ExprString(o.Expr)
			}
		}
		return e.Builder + " { " + strings.Join(ops, "; ") + " }"
	case *GoExpr:
		return "go " + ExprString(e.Expr)
	case *SelectExpr:
		cases := make([]string, len(e.Cases))
		for i, c := range e.Cases {
			cases[i] = "case " + c.Bind + " = " + ExprString(c.Recv) + " -> " + ExprString(c.Body)
		}
		s := "select { " + strings.Join(cases, "; ")
		if e.Default != nil {
			s += "; default -> " + ExprString(e.Default)
		}
		return s + " }"
	case *UsingExpr:
		return "using " + patternString(e.Pattern) + " = " + ExprString(e.Expr) + " in " + ExprString(e.Body)
	case *RegionExpr:
		ops := make([]string, len(e.Ops))
		for i, op := range e.Ops {
			switch o := op.(type) {
			case *LetBangOp:
				ops[i] = fmt.Sprintf("let! %s = %s", patternString(o.Pattern), ExprString(o.Expr))
			case *DoBangOp:
				ops[i] = fmt.Sprintf("do! %s", ExprString(o.Expr))
			case *LetOp:
				ops[i] = fmt.Sprintf("let %s = %s", patternString(o.Pattern), ExprString(o.Expr))
			case *ReturnOp:
				ops[i] = fmt.Sprintf("return %s", ExprString(o.Expr))
			case *ReturnBangOp:
				ops[i] = fmt.Sprintf("return! %s", ExprString(o.Expr))
			case *BodyOp:
				ops[i] = ExprString(o.Expr)
			}
		}
		return "region { " + strings.Join(ops, "; ") + " }"
	default:
		return "<unknown expr>"
	}
}

// patternString converts a Pattern to a human-readable string.
func patternString(p Pattern) string {
	switch p := p.(type) {
	case *WildcardPattern:
		return "_"
	case *IdentPattern:
		return p.Name
	case *LitPattern:
		return fmt.Sprintf("%v", p.Value)
	case *ConstructorPattern:
		if p.Arg != nil {
			return p.Name + "(" + patternString(p.Arg) + ")"
		}
		return p.Name
	case *RecordPattern:
		parts := make([]string, len(p.Fields))
		for i, f := range p.Fields {
			if f.Pattern != nil {
				parts[i] = f.Name + " = " + patternString(f.Pattern)
			} else {
				parts[i] = f.Name
			}
		}
		return "{" + strings.Join(parts, "; ") + "}"
	case *TuplePattern:
		parts := make([]string, len(p.Elems))
		for i, el := range p.Elems {
			parts[i] = patternString(el)
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case *ListPattern:
		if len(p.Elems) == 0 {
			return "[]"
		}
		parts := make([]string, len(p.Elems))
		for i, el := range p.Elems {
			parts[i] = patternString(el)
		}
		return "[" + strings.Join(parts, "; ") + "]"
	case *ConsPattern:
		return patternString(p.Head) + " :: " + patternString(p.Tail)
	case *AliasPattern:
		return patternString(p.Pattern) + " as " + p.Name
	default:
		return "<unknown pattern>"
	}
}
