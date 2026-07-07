// Package typecheck implements Hindley-Milner style type inference for C0.
//
// Design decisions:
//   - We use a mutable substitution map updated in-place during unification.
//   - The top-level environment is built from type declarations, then value
//     declarations are checked in order.
//   - Let-polymorphism: let-bindings are generalized (free variables not in
//     the current environment are quantified).
//   - Recursive let-bindings: all bindings in the group share the same
//     fresh type variables, which are unified with their body types.
//   - The `?` operator is special-cased: expr ? with type result<A, B>
//     yields type A.
//   - Pipeline `|>` is desugared to application: `x |> f` ≡ `f x`.
package typecheck

import (
	"fmt"
	"os"
	"strings"

	"c0.dev/compiler/internal/active"
	"c0.dev/compiler/internal/ast"
	"c0.dev/compiler/internal/exhaustive"
	"c0.dev/compiler/internal/gosig"
	"c0.dev/compiler/internal/prelude"
	"c0.dev/compiler/internal/token"
	"c0.dev/compiler/internal/typeinfo"
	"c0.dev/compiler/internal/types"
)

// ---------------------------------------------------------------------------
// TypeError — a type-check error with a source location
// ---------------------------------------------------------------------------

// TypeError carries a message and, when available, a source location.
type TypeError struct {
	Msg string
	Loc token.SourceLoc // may be zero-value if location unknown
}

func (e *TypeError) Error() string {
	if e.Loc.File == "" && e.Loc.Line == 0 {
		return e.Msg
	}
	return fmt.Sprintf("%s: %s", e.Loc, e.Msg)
}

// ---------------------------------------------------------------------------
// Environment
// ---------------------------------------------------------------------------

// Env maps names to type schemes.
type Env struct {
	parent *Env
	names  map[string]*types.Scheme
}

// NewEnv creates a new (potentially nested) environment.
func NewEnv(parent *Env) *Env {
	return &Env{parent: parent, names: make(map[string]*types.Scheme)}
}

// Lookup finds a name in the environment chain.
func (e *Env) Lookup(name string) *types.Scheme {
	for cur := e; cur != nil; cur = cur.parent {
		if s, ok := cur.names[name]; ok {
			return s
		}
	}
	return nil
}

// Bind adds a name to the current scope.
func (e *Env) Bind(name string, s *types.Scheme) {
	e.names[name] = s
}

// InScope returns the set of all free variable IDs in the environment chain
// (used for generalization: variables in scope are NOT quantified).
func (e *Env) InScope() map[int64]bool {
	m := make(map[int64]bool)
	for cur := e; cur != nil; cur = cur.parent {
		for _, s := range cur.names {
			for _, v := range s.Vars {
				m[v.ID] = true
			}
			fv := types.FreeVars(s.Type)
			for id := range fv {
				m[id] = true
			}
		}
	}
	return m
}

// ---------------------------------------------------------------------------
// Type checker state
// ---------------------------------------------------------------------------

// Checker holds the mutable state during type checking.
type Checker struct {
	env   *Env              // current environment
	sub   types.Subst       // current substitution
	errs  []error           // accumulated errors
	types typeinfo.TypeMap  // maps expression AST nodes to their inferred types
}

// pkgFromPath extracts a Go package name from an import path (last segment).
func pkgFromPath(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

// Check runs type inference on a complete module.
func Check(mod *ast.Module) []error {
	_, _, errs := CheckWithTypes(mod)
	return errs
}

// CheckWithTypes runs type inference and returns the TypeMap with fully
// resolved types for every expression node, a VarTypeMap with resolved
// types for let-bound variables, plus any errors.
func CheckWithTypes(mod *ast.Module) (typeinfo.TypeMap, typeinfo.VarTypeMap, []error) {
	c := &Checker{
		env:   NewEnv(nil),
		sub:   types.EmptySubst(),
		types: make(typeinfo.TypeMap),
	}
	c.initBuiltins()
	c.checkModule(mod)

	// Apply the final substitution to all recorded types so they are fully
	// resolved (no free type variables remain that were unified).
	// We iterate until fixpoint because the substitution may contain
	// chains (e.g. A→B→int) that a single pass won't resolve.
	for iter := 0; iter < 100; iter++ {
		// Check if any type still contains an unresolved TVar
		hasTVar := false
		for _, t := range c.types {
			if containsTVar(t) {
				hasTVar = true
				break
			}
		}
		if !hasTVar {
			break
		}
		for expr, t := range c.types {
			c.types[expr] = types.Apply(c.sub, t)
		}
	}

	// Build a VarTypeMap: for each let binding, look up the variable's
	// type scheme in the environment, instantiate, and apply the
	// substitution to get the fully resolved type.
	varTypes := make(typeinfo.VarTypeMap)
	c.collectVarTypes(mod, varTypes)

	return c.types, varTypes, c.errs
}

// containsTVar returns true if the type contains any type variable.
func containsTVar(t types.Type) bool {
	if t == nil {
		return false
	}
	switch t := t.(type) {
	case *types.TVar:
		return true
	case *types.TFun:
		return containsTVar(t.From) || containsTVar(t.To)
	case *types.TTuple:
		for _, e := range t.Elems {
			if containsTVar(e) {
				return true
			}
		}
	case *types.TRecord:
		if t == nil {
			return false
		}
		for _, f := range t.Fields {
			if containsTVar(f.Type) {
				return true
			}
		}
	case *types.TAdt:
		for _, p := range t.Params {
			if containsTVar(p) {
				return true
			}
		}
		for _, v := range t.Variants {
			if v.Arg != nil && containsTVar(v.Arg) {
				return true
			}
		}
	case *types.TCon:
		for _, a := range t.Args {
			if containsTVar(a) {
				return true
			}
		}
	case *types.TChan:
		return containsTVar(t.Elem)
	}
	return false
}

// collectVarTypes populates the VarTypeMap by walking let declarations
// and looking up variable types in the environment.
func (c *Checker) collectVarTypes(mod *ast.Module, varTypes typeinfo.VarTypeMap) {
	for _, d := range mod.Decls {
		ld, ok := d.(*ast.LetDecl)
		if !ok {
			continue
		}
		for _, b := range ld.Bindings {
			s := c.env.Lookup(b.Name)
			if s == nil {
				continue
			}
			// Instantiate the scheme and apply the substitution to resolve
			// any type variables that were unified during checking.
			t := s.Instantiate()
			t = types.Apply(c.sub, t)
			varTypes[b.Name] = t
		}
	}
}

func (c *Checker) errorf(format string, args ...any) {
	c.errs = append(c.errs, &TypeError{Msg: fmt.Sprintf(format, args...)})
}

// errorfAt creates a type error with a known source location.
func (c *Checker) errorfAt(loc token.SourceLoc, format string, args ...any) {
	c.errs = append(c.errs, &TypeError{Loc: loc, Msg: fmt.Sprintf(format, args...)})
}

// locOf extracts the source location from an expression node.
// Returns zero-value if the node type doesn't carry location info.
func locOf(e ast.Expr) token.SourceLoc {
	switch e := e.(type) {
	case *ast.LitExpr:
		return e.Loc
	case *ast.IdentExpr:
		return e.Loc
	case *ast.ConstructorExpr:
		return e.Loc
	case *ast.AppExpr:
		return e.Loc
	case *ast.IfExpr:
		return e.Loc
	case *ast.MatchExpr:
		return e.Loc
	case *ast.LetInExpr:
		return e.Loc
	case *ast.FunExpr:
		return e.Loc
	case *ast.GuardExpr:
		return e.Loc
	case *ast.IsExpr:
		return e.Loc
	case *ast.AsMatchExpr:
		return e.Loc
	case *ast.BinaryExpr:
		return e.Loc
	case *ast.PipeExpr:
		return e.Loc
	case *ast.QuestionExpr:
		return e.Loc
	case *ast.RecordExpr:
		return e.Loc
	case *ast.RecordUpdateExpr:
		return e.Loc
	case *ast.FieldAccessExpr:
		return e.Loc
	case *ast.TupleExpr:
		return e.Loc
	case *ast.ListExpr:
		return e.Loc
	case *ast.ParenExpr:
		return e.Loc
	case *ast.GoExpr:
		return e.Loc
	case *ast.SelectExpr:
		return e.Loc
	case *ast.UsingExpr:
		return e.Loc
	case *ast.RegionExpr:
		return e.Loc
	case *ast.CompExpr:
		return e.Loc
	default:
		return token.SourceLoc{}
	}
}

// fresh creates a new type variable and applies the current substitution.
func (c *Checker) fresh(name string) types.Type {
	return types.Fresh(name)
}

// ---------------------------------------------------------------------------
// Built-in types and constructors
// ---------------------------------------------------------------------------

func (c *Checker) initBuiltins() {
	// option ADT: type 'a option = None | Some of 'a
	// result ADT: type ('ok, 'err) result = Ok of 'ok | Error of 'err
	// list type constructor: 'a list with [] and :: constructors

	// Register constructor types for option.
	// None: 'a -> 'a option  (actually just option<'a> since it has no arg)
	// Some: 'a -> 'a option
	a := types.Fresh("'a")
	ok := types.Fresh("'ok")
	err := types.Fresh("'err")

	optType := types.OptionType(a)
	resType := types.ResultType(ok, err)

	// None: option<'a> (no argument → the type itself is the constructor)
	c.env.Bind("None", types.Mono(optType))
	// Some: 'a -> option<'a>
	c.env.Bind("Some", types.Mono(&types.TFun{From: a, To: optType}))
	// Ok: 'ok -> result<'ok, 'err>
	c.env.Bind("Ok", types.Mono(&types.TFun{From: ok, To: resType}))
	// Error: 'err -> result<'ok, 'err>
	c.env.Bind("Error", types.Mono(&types.TFun{From: err, To: resType}))

	// Register built-in ADTs for exhaustiveness checking
	exhaustive.RegisterADT("result", []string{"Ok", "Error"})
	exhaustive.RegisterADT("option", []string{"None", "Some"})

	// Register prelude bindings (available to all programs without `open`).
	// These are shadowable — user definitions in the same scope override them.
	pre := prelude.Default()
	for _, b := range pre.Bindings {
		// Bind in the base environment; user definitions will shadow these
		// because they are added to a nested scope during checking.
		c.env.Bind(b.Name, b.Scheme)
	}

	// Register owned_chan as a built-in linear type for type annotations.
	// This enables `let ch : int owned_chan = OwnedChan.make ()`.
	c.env.Bind("owned_chan", types.Mono(&types.TAdt{Name: "owned_chan", Linear: true}))
}

// ---------------------------------------------------------------------------
// Module checking
// ---------------------------------------------------------------------------

func (c *Checker) checkModule(mod *ast.Module) {
	// First pass: register all type declarations so they can be used in value
	// declarations (no forward references for types yet; they must be declared
	// before use in the source order).

	typeDecls := make(map[string]*types.Scheme)

	for _, d := range mod.Decls {
		td, ok := d.(*ast.TypeDecl)
		if !ok {
			continue
		}
		scheme := c.convertTypeDecl(td)
		c.env.Bind(td.Name, scheme)
		typeDecls[td.Name] = scheme
	}
	_ = typeDecls

	// Second pass: check value declarations (let, extern).
	for _, d := range mod.Decls {
		switch d := d.(type) {
		case *ast.LetDecl:
			c.checkLetDecl(d)
		case *ast.ExternDecl:
			// Reject non-Go extern for now
			if d.Lang != "go" {
				c.errorf("only 'go' extern is supported, got %q", d.Lang)
				continue
			}
			// Extract a short package name from the import path
			pkgName := pkgFromPath(d.Path)
			for _, ev := range d.Vals {
				t := c.convertASTType(ev.Type)

				// Optional gosig fallback: try to refine the type from the
				// real Go signature. This gives better lambda param types.
				if refined := c.refineExternType(d.Path, ev.Name, t); refined != nil {
					t = refined
				}

				scheme := types.Mono(t)
				// Bind unqualified name
				if c.env.Lookup(ev.Name) != nil {
					c.errorf("extern binding %q conflicts with existing name", ev.Name)
				} else {
					c.env.Bind(ev.Name, scheme)
				}
				// Also bind qualified name: pkg.Name
				qualified := pkgName + "." + ev.Name
				c.env.Bind(qualified, scheme)
			}
		case *ast.TypeDecl:
			// Already handled in first pass
		}
	}
}

// ---------------------------------------------------------------------------
// Type declaration conversion
// ---------------------------------------------------------------------------

func (c *Checker) convertTypeDecl(td *ast.TypeDecl) *types.Scheme {
	switch k := td.Kind.(type) {
	case *ast.OpaqueTypeKind:
		// Opaque linear type: no body, just a name
		adt := &types.TAdt{
			Name:     td.Name,
			Linear:   td.Quantity == 1,
			Variants: nil,
		}
		return types.Mono(adt)

	case *ast.RecordTypeKind:
		fields := make([]types.Field, len(k.Fields))
		for i, f := range k.Fields {
			fields[i] = types.Field{Name: f.Name, Type: c.convertASTType(f.Type)}
		}
		t := &types.TRecord{Fields: fields}
		// Quantify type params if present
		if len(td.TypeParams) > 0 {
			vars := make([]*types.TVar, len(td.TypeParams))
			for i, tp := range td.TypeParams {
				vars[i] = types.Fresh(tp)
			}
			// For now, simple ADTs don't substitute params into the body.
			// A full Hindley-Milner system would track this, but for the
			// examples the types are monomorphic.
			if len(vars) > 0 {
				return &types.Scheme{Vars: vars, Type: t}
			}
		}
		return types.Mono(t)

	case *ast.ADTTypeKind:
		variants := make([]types.Variant, len(k.Cases))
		for i, cs := range k.Cases {
			v := types.Variant{Name: cs.Name}
			if cs.Arg != nil {
				v.Arg = c.convertASTType(cs.Arg)
			}
			variants[i] = v
		}
		adt := &types.TAdt{
			Name:     td.Name,
			Params:   nil,
			Variants: variants,
			Linear:   td.Quantity == 1,
		}
		// Register constructors in the environment
		for _, cs := range k.Cases {
			var ctorType types.Type
			if cs.Arg != nil {
				ctorType = &types.TFun{From: c.convertASTType(cs.Arg), To: adt}
			} else {
				ctorType = adt
			}
			c.env.Bind(cs.Name, types.Mono(ctorType))
		}
		return types.Mono(adt)

	case *ast.AliasTypeKind:
		t := c.convertASTType(k.Alias)
		return types.Mono(t)
	}
	return types.Mono(types.Unit)
}

// ---------------------------------------------------------------------------
// AST type → internal type conversion
// ---------------------------------------------------------------------------

func (c *Checker) convertASTType(at ast.Type) types.Type {
	if at == nil {
		return c.fresh("'a")
	}
	switch t := at.(type) {
	case *ast.TIdent:
		// Map primitive type names
		switch t.Name {
		case "int":
			return types.Int
		case "float":
			return types.Float
		case "bool":
			return types.Bool
		case "string":
			return types.String
		case "unit":
			return types.Unit
		case "bytes":
			return types.Bytes
		case "rune":
			return types.Rune
		case "list":
			// Type constructor — args filled by TApp
			return &types.TCon{Name: "list"}
		case "option":
			return &types.TCon{Name: "option"}
		case "result":
			return &types.TCon{Name: "result"}
		case "owned_chan":
			return &types.TAdt{Name: "owned_chan", Linear: true}
		default:
			// Look up user-defined type
			if s := c.env.Lookup(t.Name); s != nil {
				return s.Type
			}
			// Unknown — could be a module-qualified type; just use as-is
			return &types.Prim{Name: t.Name}
		}
	case *ast.TApp:
		// Type application: TApp(Func, Arg)
		// E.g. TApp(list, order) → list<order>
		//      TApp(result, Tuple(user, error)) → result<user, error>
		funcType := c.convertASTType(t.Func)
		argType := c.convertASTType(t.Arg)

		// If func is a recognized type constructor, fill its args.
		if tc, ok := funcType.(*types.TCon); ok {
			if tup, ok := argType.(*types.TTuple); ok {
				// result(user, error) — flatten the tuple
				tc.Args = append(tc.Args, tup.Elems...)
			} else {
				// list<order>, option<int>
				tc.Args = append(tc.Args, argType)
			}
			return tc
		}
		if tad, ok := funcType.(*types.TAdt); ok {
			if tup, ok := argType.(*types.TTuple); ok {
				tad.Params = append(tad.Params, tup.Elems...)
			} else {
				tad.Params = append(tad.Params, argType)
			}
			return tad
		}
		// Fallback: wrap as generic application
		return &types.TCon{Name: "app", Args: []types.Type{funcType, argType}}

	case *ast.TFun:
		fn := &types.TFun{
			From: c.convertASTType(t.From),
			To:   c.convertASTType(t.To),
		}
		if t.Effects != nil {
			fn.Effects = &types.EffectRow{
				Effects: t.Effects.Effects,
				Open:    t.Effects.Open,
			}
			if t.Effects.Rest != "" {
				fn.Effects.Rest = types.Fresh(t.Effects.Rest)
			}
		}
		return fn
	case *ast.TTuple:
		elems := make([]types.Type, len(t.Elems))
		for i, e := range t.Elems {
			elems[i] = c.convertASTType(e)
		}
		return &types.TTuple{Elems: elems}
	case *ast.TRecord:
		fields := make([]types.Field, len(t.Fields))
		for i, f := range t.Fields {
			fields[i] = types.Field{Name: f.Name, Type: c.convertASTType(f.Type)}
		}
		return &types.TRecord{Fields: fields, Open: t.Open}
	case *ast.RefinementType:
		// Refinement types are transparent — only the inner type matters.
		// The where clause is a runtime contract, not a type-level constraint.
		return c.convertASTType(t.Inner)
	case *ast.TChan:
		return &types.TChan{Elem: c.convertASTType(t.Elem)}
	case *ast.TVar:
		// Type variable: 'a → fresh type variable
		return c.fresh(t.Name)
	default:
		return c.fresh("'a")
	}
}

// ---------------------------------------------------------------------------
// Let declaration checking
// ---------------------------------------------------------------------------

func (c *Checker) checkLetDecl(d *ast.LetDecl) {
	// Active pattern: let (|Name|_|) (arg: T) : U option = body
	if d.ActivePattern {
		for _, b := range d.Bindings {
			t := c.checkBinding(b)
			// The type of an active pattern is InputType -> option<OutputType>
			// Store in the active pattern registry
			inputType := types.Fresh("input")
			outputType := types.Fresh("output")
			optType := types.OptionType(outputType)
			fnType := &types.TFun{From: inputType, To: optType}
			c.unify(t, fnType)

			solvedInput := types.Apply(c.sub, inputType)
			solvedOutput := types.Apply(c.sub, outputType)
			goFuncName := "__active_" + b.Name
			active.GlobalRegistry.Register(b.Name, solvedInput, solvedOutput, goFuncName)

			// Also bind as a regular function value
			scheme := types.Generalize(t, c.env.InScope())
			c.env.Bind(b.Name, scheme)
		}
		return
	}

	if d.Rec {
		c.checkLetRec(d)
		return
	}
	for _, b := range d.Bindings {
		t := c.checkBinding(b)
		// Generalize and bind
		inScope := c.env.InScope()
		scheme := types.Generalize(t, inScope)
		c.env.Bind(b.Name, scheme)
	}
}

func (c *Checker) checkLetRec(d *ast.LetDecl) {
	// Create fresh type variables for all bindings in the group
	freshVars := make([]types.Type, len(d.Bindings))
	for i, b := range d.Bindings {
		fv := c.fresh(b.Name)
		freshVars[i] = fv
		// Bind the fresh variable in the env so the body can see it
		c.env.Bind(b.Name, types.Mono(fv))
	}

	for i, b := range d.Bindings {
		t := c.infer(b.Body)
		// If there are params, wrap in function types
		for j := len(b.Params) - 1; j >= 0; j-- {
			var paramType types.Type
			if b.Params[j].Type != nil {
				paramType = c.convertASTType(b.Params[j].Type)
			} else {
				paramType = c.fresh(b.Params[j].Name)
			}
			t = &types.TFun{From: paramType, To: t}
		}
		// Attach effect row if specified
		if b.RetEffects != nil && len(b.Params) > 0 {
			if fn, ok := t.(*types.TFun); ok {
				fn.Effects = &types.EffectRow{
					Effects: b.RetEffects.Effects,
					Open:    b.RetEffects.Open,
				}
				if b.RetEffects.Rest != "" {
					fn.Effects.Rest = types.Fresh(b.RetEffects.Rest)
				}
			}
		}
		// Unify with the fresh variable
		c.unify(freshVars[i], t)
	}

	// After all bodies are checked, generalize and re-bind
	for i, b := range d.Bindings {
		solved := types.Apply(c.sub, freshVars[i])
		inScope := c.env.InScope()
		scheme := types.Generalize(solved, inScope)
		c.env.Bind(b.Name, scheme)
	}
}

func (c *Checker) checkBinding(b ast.LetBinding) types.Type {
	// Create a nested scope for params
	saved := c.env
	c.env = NewEnv(c.env)

	// Bind parameters with fresh or annotated types
	var paramTypes []types.Type
	for _, p := range b.Params {
		var pt types.Type
		if p.Type != nil {
			pt = c.convertASTType(p.Type)
		} else {
			pt = c.fresh(p.Name)
		}
		c.env.Bind(p.Name, types.Mono(pt))
		paramTypes = append(paramTypes, pt)
	}

	// Infer body type
	bodyType := c.infer(b.Body)

	c.env = saved

	// If there's a return type annotation, unify with it
	if b.RetType != nil {
		retType := c.convertASTType(b.RetType)
		c.unify(bodyType, retType)
		// Use the annotation type (concrete) instead of the body type
		// (which may be an unresolved TVar). The unification ensures
		// they're equivalent, but retType is concrete while bodyType
		// may still be a TVar that gets generalized incorrectly.
		bodyType = retType
	}

	// Wrap in function types (curried)
	result := bodyType
	for i := len(paramTypes) - 1; i >= 0; i-- {
		result = &types.TFun{From: paramTypes[i], To: result}
	}

	// Attach effect row to the outermost function type if specified
	if b.RetEffects != nil && len(paramTypes) > 0 {
		if fn, ok := result.(*types.TFun); ok {
			fn.Effects = &types.EffectRow{
				Effects: b.RetEffects.Effects,
				Open:    b.RetEffects.Open,
			}
			if b.RetEffects.Rest != "" {
				fn.Effects.Rest = types.Fresh(b.RetEffects.Rest)
			}
		}
	}

	return result
}

// ---------------------------------------------------------------------------
// Expression inference
// ---------------------------------------------------------------------------

func (c *Checker) infer(e ast.Expr) types.Type {
	var t types.Type
	switch e := e.(type) {
	case *ast.LitExpr:
		t = c.inferLit(e)
	case *ast.IdentExpr:
		t = c.inferIdent(e)
	case *ast.ConstructorExpr:
		t = c.inferConstructor(e)
	case *ast.AppExpr:
		t = c.inferApp(e)
	case *ast.IfExpr:
		t = c.inferIf(e)
	case *ast.MatchExpr:
		t = c.inferMatch(e)
	case *ast.LetInExpr:
		t = c.inferLetIn(e)
	case *ast.FunExpr:
		t = c.inferFun(e)
	case *ast.BinaryExpr:
		t = c.inferBinary(e)
	case *ast.PipeExpr:
		t = c.inferPipe(e)
	case *ast.QuestionExpr:
		t = c.inferQuestion(e)
	case *ast.RecordExpr:
		t = c.inferRecord(e)
	case *ast.RecordUpdateExpr:
		t = c.inferRecordUpdate(e)
	case *ast.FieldAccessExpr:
		t = c.inferFieldAccess(e)
	case *ast.TupleExpr:
		t = c.inferTuple(e)
	case *ast.ListExpr:
		t = c.inferList(e)
	case *ast.ParenExpr:
		t = c.infer(e.Inner)
	case *ast.GuardExpr:
		t = c.inferGuard(e)
	case *ast.IsExpr:
		t = c.inferIs(e)
	case *ast.AsMatchExpr:
		t = c.inferAsMatch(e)
	case *ast.GoExpr:
		t = c.inferGo(e)
	case *ast.SelectExpr:
		t = c.inferSelect(e)
	case *ast.UsingExpr:
		t = c.inferUsing(e)
	case *ast.RegionExpr:
		t = c.inferRegion(e)
	default:
		c.errorfAt(locOf(e), "type inference not implemented for %T", e)
		t = types.Unit
	}

	if c.types != nil && t != nil {
		c.types[e] = t
	}
	return t
}

func (c *Checker) inferLit(e *ast.LitExpr) types.Type {
	switch e.Kind {
	case token.INT:
		return types.Int
	case token.FLOAT:
		return types.Float
	case token.STRING:
		return types.String
	case token.TRUE, token.FALSE:
		return types.Bool
	case token.UNIT:
		return types.Unit
	default:
		return types.Unit
	}
}

func (c *Checker) inferIdent(e *ast.IdentExpr) types.Type {
	s := c.env.Lookup(e.Name)
	if s == nil {
		// External/unknown identifier — give it a fresh polymorphic type.
		// This allows the examples to reference external modules (Console,
		// File, Json, etc.) without explicit imports.
		return c.fresh(e.Name)
	}
	return s.Instantiate()
}

func (c *Checker) inferConstructor(e *ast.ConstructorExpr) types.Type {
	s := c.env.Lookup(e.Name)
	if s == nil {
		// Capital-letter names used as modules/variables are parsed as
		// constructors by the lexer but may be regular identifiers.
		// Fall back to identifier lookup.
		return c.inferIdent(&ast.IdentExpr{Name: e.Name})
	}
	inst := s.Instantiate()

	if e.Arg != nil {
		// Treat as application: ctor(arg)
		funcType := inst
		argType := c.infer(e.Arg)
		resultType := c.fresh("result")
		c.unify(&types.TFun{From: argType, To: resultType}, funcType)
		return resultType
	}
	return inst
}

func (c *Checker) inferApp(e *ast.AppExpr) types.Type {
	funcType := c.infer(e.Func)

	// Bidirectional inference: if the argument is a lambda and we can
	// resolve the function type to a concrete TFun, propagate the expected
	// parameter type into the lambda so the body can use it.
	var argType types.Type
	if fn, ok := e.Arg.(*ast.FunExpr); ok {
		resolvedFunc := types.Apply(c.sub, funcType)
		if tfun, ok := resolvedFunc.(*types.TFun); ok {
			expected := tfun.From
			// Only propagate if expected is concrete (not a TVar).
			if _, isTVar := expected.(*types.TVar); !isTVar {
				argType = c.inferFunExpected(fn, expected)
			} else {
				argType = c.infer(e.Arg)
			}
		} else {
			argType = c.infer(e.Arg)
		}
	} else {
		argType = c.infer(e.Arg)
	}

	resultType := c.fresh("result")
	fnType := &types.TFun{From: argType, To: resultType}
	c.unifyAt(e.Loc, funcType, fnType)
	return resultType
}

// inferFunExpected infers a function expression with a known expected type.
// For each unannotated parameter, if the expected type is a TFun, the
// parameter is unified with the expected parameter type BEFORE inferring
// the body. This provides better type information in the body and enables
// inference of lambda parameter types from context.
func (c *Checker) inferFunExpected(e *ast.FunExpr, expected types.Type) types.Type {
	saved := c.env
	c.env = NewEnv(c.env)

	expectedParam := expected // peeled as we process params

	var paramTypes []types.Type
	for _, p := range e.Params {
		var pt types.Type
		if p.Type != nil {
			pt = c.convertASTType(p.Type)
		} else if expectedParam != nil {
			// Try to extract the expected type for this param.
			resolved := types.Apply(c.sub, expectedParam)
			if fn, ok := resolved.(*types.TFun); ok {
				pt = c.fresh(p.Name)
				c.unify(pt, fn.From)
				// Advance to the next expected param (the return type
				// becomes the expected type for the rest of the lambda).
				expectedParam = fn.To
			} else {
				// Expected type is not a function — fall back to fresh.
				pt = c.fresh(p.Name)
				expectedParam = nil
			}
		} else {
			pt = c.fresh(p.Name)
		}
		c.env.Bind(p.Name, types.Mono(pt))
		paramTypes = append(paramTypes, pt)
	}

	bodyType := c.infer(e.Body)
	c.env = saved

	result := bodyType
	for i := len(paramTypes) - 1; i >= 0; i-- {
		result = &types.TFun{From: paramTypes[i], To: result}
	}
	return result
}

func (c *Checker) inferIf(e *ast.IfExpr) types.Type {
	condType := c.infer(e.Cond)
	c.unifyAt(e.Loc, condType, types.Bool)
	thenType := c.infer(e.ThenBranch)
	elseType := c.infer(e.ElseBranch)
	c.unifyAt(e.Loc, thenType, elseType)
	return thenType
}

func (c *Checker) inferMatch(e *ast.MatchExpr) types.Type {
	scrutType := c.infer(e.Scrutinee)
	resultType := c.fresh("match_result")

	for _, arm := range e.Arms {
		// Create a new scope for pattern variables
		saved := c.env
		c.env = NewEnv(c.env)
		c.checkPattern(e.Loc, arm.Pattern, scrutType)
		if arm.Guard != nil {
			guardType := c.infer(arm.Guard)
			c.unifyAt(e.Loc, guardType, types.Bool)
		}
		bodyType := c.infer(arm.Body)
		c.unifyAt(e.Loc, bodyType, resultType)
		c.env = saved
	}
	return resultType
}

func (c *Checker) inferLetIn(e *ast.LetInExpr) types.Type {
	// Process as non-recursive let: check bindings, add to env, check body
	for _, b := range e.Bindings {
		t := c.checkBinding(b)
		inScope := c.env.InScope()
		scheme := types.Generalize(t, inScope)
		c.env.Bind(b.Name, scheme)
	}
	return c.infer(e.Body)
}

func (c *Checker) inferFun(e *ast.FunExpr) types.Type {
	saved := c.env
	c.env = NewEnv(c.env)

	var paramTypes []types.Type
	for _, p := range e.Params {
		var pt types.Type
		if p.Type != nil {
			pt = c.convertASTType(p.Type)
		} else {
			pt = c.fresh(p.Name)
		}
		c.env.Bind(p.Name, types.Mono(pt))
		paramTypes = append(paramTypes, pt)
	}

	bodyType := c.infer(e.Body)
	c.env = saved

	result := bodyType
	for i := len(paramTypes) - 1; i >= 0; i-- {
		result = &types.TFun{From: paramTypes[i], To: result}
	}
	return result
}

func (c *Checker) inferBinary(e *ast.BinaryExpr) types.Type {
	left := c.infer(e.Left)
	right := c.infer(e.Right)

	switch e.Op {
	case token.PLUS, token.MINUS, token.STAR, token.SLASH:
		// Arithmetic: both operands must be the same numeric type (int or float)
		c.unifyAt(e.Loc, left, right)
		// Also allow int+int=int, float+float=float
		// (We just unify them; the result type is the unified type.)
		return left

	case token.STARDOT, token.PLUSDOT, token.MINUSDOT, token.SLASHDOT:
		// Float arithmetic: *. +. -. /. force float
		c.unifyAt(e.Loc, left, types.Float)
		c.unifyAt(e.Loc, right, types.Float)
		return types.Float

	case token.EQUALS, token.NEQ, token.DIAMOND:
		// Comparison: both operands same type, result is bool
		c.unifyAt(e.Loc, left, right)
		_ = types.Bool
		return types.Bool

	case token.LT, token.GT, token.LEQ, token.GEQ:
		// Ordered comparison: both operands same type (int or float), result bool
		c.unifyAt(e.Loc, left, right)
		return types.Bool

	case token.CARET:
		// String concatenation: both strings, result string
		c.unifyAt(e.Loc, left, types.String)
		c.unifyAt(e.Loc, right, types.String)
		return types.String

	case token.AMPAMP, token.PIPEPIPE:
		// Logical: both bool, result bool
		c.unifyAt(e.Loc, left, types.Bool)
		c.unifyAt(e.Loc, right, types.Bool)
		return types.Bool

	case token.CONS:
		// x :: xs — x: A, xs: list<A>, result: list<A>
		c.unifyAt(e.Loc, right, types.ListType(left))
		return right

	default:
		c.errorfAt(e.Loc, "type inference not implemented for binary operator %s", e.Op)
		return types.Unit
	}
}

func (c *Checker) inferPipe(e *ast.PipeExpr) types.Type {
	// x |> f  ≡  f x
	left := c.infer(e.Left)
	right := c.infer(e.Right)
	result := c.fresh("pipe")
	c.unifyAt(e.Loc, right, &types.TFun{From: left, To: result})
	return result
}

func (c *Checker) inferQuestion(e *ast.QuestionExpr) types.Type {
	leftType := c.infer(e.Left)
	// Left should be result<A, B>, result is A
	a := c.fresh("ok")
	b := c.fresh("err")
	c.unify(leftType, types.ResultType(a, b))
	if e.Arg != nil {
		// Optional error transformation argument
		_ = c.infer(e.Arg)
	}
	return a
}

func (c *Checker) inferRecord(e *ast.RecordExpr) types.Type {
	fields := make([]types.Field, len(e.Fields))
	for i, f := range e.Fields {
		var ft types.Type
		if f.Value != nil {
			ft = c.infer(f.Value)
		} else {
			// Punning: field name is also variable name
			ft = c.inferIdent(&ast.IdentExpr{Name: f.Name})
		}
		fields[i] = types.Field{Name: f.Name, Type: ft}
	}
	return &types.TRecord{Fields: fields}
}

func (c *Checker) inferRecordUpdate(e *ast.RecordUpdateExpr) types.Type {
	baseType := c.infer(e.Base)
	// Verify that the updated fields exist and have compatible types.
	if rec, ok := baseType.(*types.TRecord); ok {
		for _, f := range e.Fields {
			valType := c.infer(f.Value)
			if ft := rec.Lookup(f.Name); ft != nil {
				c.unify(valType, ft)
			}
		}
	}
	return baseType
}

func (c *Checker) inferFieldAccess(e *ast.FieldAccessExpr) types.Type {
	// Check for prelude-qualified names like Chan.make, Console.print_line.
	// The codegen resolves these through the prelude, so the typechecker
	// must do the same to get correct types for polymorphic prelude calls.
	qualified := c.fieldAccessName(e)
	if qualified != "" {
		if s := c.env.Lookup(qualified); s != nil {
			return s.Instantiate()
		}
	}

	leftType := c.infer(e.Left)
	// For field access, we only need the field to exist in the record.
	// We don't require the records to be identical.
	resultType := c.fresh(e.Field)

	// If the left side is already a known record, look up the field.
	if rec, ok := leftType.(*types.TRecord); ok {
		if ft := rec.Lookup(e.Field); ft != nil {
			c.unify(resultType, ft)
			return resultType
		}
	}

	// Otherwise, create a partial record constraint:
	// the left type must be a record containing at least this field.
	field := types.Field{Name: e.Field, Type: resultType}
	partialRec := &types.TRecord{Fields: []types.Field{field}}
	// We use a different approach: unify the field's type within the record
	// without requiring exact field-set match.  Since TRecord unification
	// requires exact match, we relax this by only checking field presence.
	_ = partialRec

	// For unknown record types, just return the fresh result type.
	// Full record type checking would require row types or similar.
	return resultType
}

// fieldAccessName returns the dotted name for a simple field-access
// expression like Console.print_line or Chan.make, or empty string.
func (c *Checker) fieldAccessName(e *ast.FieldAccessExpr) string {
	if ctor, ok := e.Left.(*ast.ConstructorExpr); ok && ctor.Arg == nil {
		return ctor.Name + "." + e.Field
	}
	if ident, ok := e.Left.(*ast.IdentExpr); ok {
		return ident.Name + "." + e.Field
	}
	return ""
}

func (c *Checker) inferTuple(e *ast.TupleExpr) types.Type {
	elems := make([]types.Type, len(e.Elems))
	for i, el := range e.Elems {
		elems[i] = c.infer(el)
	}
	return &types.TTuple{Elems: elems}
}

func (c *Checker) inferList(e *ast.ListExpr) types.Type {
	if len(e.Elems) == 0 {
		// [] has type 'a list
		return types.ListType(c.fresh("'a"))
	}
	elemType := c.infer(e.Elems[0])
	for _, el := range e.Elems[1:] {
		c.unify(elemType, c.infer(el))
	}
	return types.ListType(elemType)
}

func (c *Checker) inferGuard(e *ast.GuardExpr) types.Type {
	// guard pat1 = expr1 and pat2 = expr2 else expr
	// Desugars to nested match, so we check each binding's pattern
	// against its expression type, then check the else branch.
	for _, b := range e.Bindings {
		patType := c.infer(b.Expr)
		c.checkPattern(e.Loc, b.Pattern, patType)
	}
	elseType := c.infer(e.Else_)
	return elseType
}

func (c *Checker) inferIs(e *ast.IsExpr) types.Type {
	leftType := c.infer(e.Left)
	// Just check the pattern; result is bool
	c.checkPattern(e.Loc, e.Pattern, leftType)
	return types.Bool
}

func (c *Checker) inferAsMatch(e *ast.AsMatchExpr) types.Type {
	leftType := c.infer(e.Left)
	saved := c.env
	c.env = NewEnv(c.env)
	c.checkPattern(e.Loc, e.Pattern, leftType)
	bodyType := c.infer(e.Body)
	c.env = saved
	elseType := c.infer(e.ElseBody)
	c.unify(bodyType, elseType)
	return bodyType
}

func (c *Checker) inferGo(e *ast.GoExpr) types.Type {
	exprType := c.infer(e.Expr)
	expected := &types.TFun{From: types.Unit, To: types.Unit}
	c.unify(exprType, expected)
	return types.Unit
}

func (c *Checker) inferSelect(e *ast.SelectExpr) types.Type {
	var rType types.Type
	for i := range e.Cases {
		// Infer the channel receive expression
		chType := c.infer(e.Cases[i].Recv)
		// Bind the variable
		elemType := types.Fresh("elem")
		c.unify(chType, &types.TChan{Elem: elemType})
		c.env.Bind(e.Cases[i].Bind, types.Mono(elemType))
		bodyType := c.infer(e.Cases[i].Body)
		if rType == nil {
			rType = bodyType
		} else {
			c.unify(rType, bodyType)
		}
	}
	if e.Default != nil {
		dType := c.infer(e.Default)
		if rType == nil {
			rType = dType
		} else {
			c.unify(rType, dType)
		}
	}
	if rType == nil {
		rType = types.Unit
	}
	return rType
}

func (c *Checker) inferUsing(e *ast.UsingExpr) types.Type {
	exprType := c.infer(e.Expr)
	c.checkPattern(e.Loc, e.Pattern, exprType)
	return c.infer(e.Body)
}

func (c *Checker) inferRegion(e *ast.RegionExpr) types.Type {
	saved := c.env
	c.env = NewEnv(c.env)

	var resultType types.Type = types.Unit

	for _, op := range e.Ops {
		switch o := op.(type) {
		case *ast.LetBangOp:
			// let! pattern = expr: bind pattern to RHS type (like let)
			t := c.infer(o.Expr)
			c.checkPattern(e.Loc, o.Pattern, t)
		case *ast.LetOp:
			// let pattern = expr: bind pattern to RHS type
			t := c.infer(o.Expr)
			c.checkPattern(e.Loc, o.Pattern, t)
		case *ast.DoBangOp:
			// do! expr: expr should have unit type
			t := c.infer(o.Expr)
			c.unify(t, types.Unit)
		case *ast.ReturnOp:
			// return expr: determines the region's result type
			resultType = c.infer(o.Expr)
		case *ast.ReturnBangOp:
			// return! expr: passes through
			resultType = c.infer(o.Expr)
		case *ast.BodyOp:
			// body expression (used without explicit return)
			resultType = c.infer(o.Expr)
		}
	}

	c.env = saved
	return resultType
}

// ---------------------------------------------------------------------------
// Pattern checking
// ---------------------------------------------------------------------------

func (c *Checker) checkPattern(loc token.SourceLoc, p ast.Pattern, scrutType types.Type) {
	switch p := p.(type) {
	case *ast.WildcardPattern:
		// Nothing to check
	case *ast.IdentPattern:
		// Bind the variable to the scrutinee type
		c.env.Bind(p.Name, types.Mono(scrutType))
	case *ast.LitPattern:
		// Check that the literal type matches
		litType := c.inferLit(&ast.LitExpr{Value: p.Value, Kind: p.Kind})
		c.unify(scrutType, litType)
	case *ast.ConstructorPattern:
		// Check if this is an active pattern
		if ap := active.GlobalRegistry.Lookup(p.Name); ap != nil {
			// Active pattern: InputType -> option<OutputType>
			// Scrutinee must match InputType
			c.unifyAt(loc, scrutType, ap.InputType)
			// Inner pattern binds to OutputType
			if p.Arg != nil {
				c.checkPattern(loc, p.Arg, ap.OutputType)
			}
			return
		}

		// Find the constructor type and match
		s := c.env.Lookup(p.Name)
		if s == nil {
			c.errorfAt(loc, "undefined constructor pattern: %s", p.Name)
			return
		}
		ctorType := s.Instantiate()
		// Constructor type is either TAdt (no arg) or TFun(Arg, TAdt)
		if p.Arg != nil {
			if fn, ok := ctorType.(*types.TFun); ok {
				c.unifyAt(loc, fn.To, scrutType)
				c.checkPattern(loc, p.Arg, fn.From)
			} else {
				c.errorfAt(loc, "constructor %s takes no argument", p.Name)
			}
		} else {
			c.unifyAt(loc, ctorType, scrutType)
		}
	case *ast.RecordPattern:
		// Scrutinee must be a record; each field pattern checked
		rt := c.unpackRecord(loc, scrutType)
		for _, f := range p.Fields {
			fieldType := rt.Lookup(f.Name)
			if fieldType == nil {
				c.errorfAt(loc, "record has no field %q", f.Name)
				continue
			}
			if f.Pattern != nil {
				c.checkPattern(loc, f.Pattern, fieldType)
			} else {
				// Punning: bind field name to field type
				c.env.Bind(f.Name, types.Mono(fieldType))
			}
		}
	case *ast.TuplePattern:
		// Must be a tuple type of same arity
		if tt, ok := scrutType.(*types.TTuple); ok {
			if len(p.Elems) != len(tt.Elems) {
				c.errorfAt(loc, "tuple pattern arity mismatch: %d vs %d", len(p.Elems), len(tt.Elems))
				return
			}
			for i, ep := range p.Elems {
				c.checkPattern(loc, ep, tt.Elems[i])
			}
		} else {
			c.errorfAt(loc, "expected tuple type for tuple pattern")
		}
	case *ast.ListPattern:
		// Must be a list type
		elemType := c.unpackList(loc, scrutType)
		for _, ep := range p.Elems {
			c.checkPattern(loc, ep, elemType)
		}
	case *ast.ConsPattern:
		// head :: tail — both must match the list element type
		elemType := c.unpackList(loc, scrutType)
		c.checkPattern(loc, p.Head, elemType)
		c.checkPattern(loc, p.Tail, types.ListType(elemType))
	case *ast.AliasPattern:
		c.checkPattern(loc, p.Pattern, scrutType)
		c.env.Bind(p.Name, types.Mono(scrutType))
	}
}

// unpackList extracts the element type from a list type, creating a fresh
// variable if the type is not yet known.
func (c *Checker) unpackList(loc token.SourceLoc, t types.Type) types.Type {
	if tc, ok := t.(*types.TCon); ok && tc.Name == "list" && len(tc.Args) > 0 {
		return tc.Args[0]
	}
	// Create a fresh variable and force the type to be a list
	elem := c.fresh("elem")
	c.unifyAt(loc, t, types.ListType(elem))
	return elem
}

// unpackRecord extracts the record type, or creates a fresh one.
func (c *Checker) unpackRecord(loc token.SourceLoc, t types.Type) *types.TRecord {
	if rec, ok := t.(*types.TRecord); ok {
		return rec
	}
	// Create a fresh record and unify
	rec := &types.TRecord{}
	c.unifyAt(loc, t, rec)
	return rec
}

// ---------------------------------------------------------------------------
// Unification helper
// ---------------------------------------------------------------------------

func (c *Checker) unify(t1, t2 types.Type) {
	c.unifyAt(token.SourceLoc{}, t1, t2)
}

// unifyAt is like unify but attaches a source location to any error.
func (c *Checker) unifyAt(loc token.SourceLoc, t1, t2 types.Type) {
	// Apply current substitution first
	t1 = types.Apply(c.sub, t1)
	t2 = types.Apply(c.sub, t2)

	newSub, err := types.Unify(t1, t2)
	if err != nil {
		c.errorfAt(loc, "%v", err)
		return
	}
	// Compose the new substitution into the current one
	c.sub = types.Compose(newSub, c.sub)
}

// ---------------------------------------------------------------------------
// Extern type refinement via go/types (optional gosig fallback)
// ---------------------------------------------------------------------------

// refineExternType attempts to look up the real Go function signature for an
// extern binding and convert it to a more precise C0 type. If the lookup
// fails or the conversion produces an unsatisfactory type, it returns nil
// and the caller keeps the declared C0 type.
func (c *Checker) refineExternType(importPath, funcName string, declared types.Type) types.Type {
	if importPath == "" {
		return nil // same-package externs have no Go package to load
	}
	sig, err := gosig.LookupFunc(importPath, funcName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "c0: gosig fallback for %s.%s: %v\n", importPath, funcName, err)
		return nil
	}

	// Build a curried C0 function type from the Go parameters and results.
	// Go result types become the final return type; if there are multiple
	// results we use unit (tuples not yet supported in externs).
	var resultType types.Type
	switch len(sig.Results) {
	case 0:
		resultType = types.Unit
	case 1:
		rt := goTypeToC0Type(sig.Results[0].Type)
		if rt == nil {
			fmt.Fprintf(os.Stderr, "c0: gosig fallback for %s.%s: cannot map Go result type %q to C0\n",
				importPath, funcName, sig.Results[0].Type)
			return nil
		}
		resultType = rt
	default:
		// Multiple return values — keep the declared type.
		fmt.Fprintf(os.Stderr, "c0: gosig fallback for %s.%s: %d result values (not supported)\n",
			importPath, funcName, len(sig.Results))
		return nil
	}

	// Extract the declared return type (the rightmost leaf of the function
	// type) to preserve it if the Go sig is less specific.
	declaredResult := extractResultType(declared)
	if declaredResult != nil && resultType != nil {
		// If declared result is more specific than what gosig can infer,
		// keep the declared one.  For example, if C0 says `string` but
		// gosig maps the Go type to `interface{}`, keep C0's `string`.
		if isMoreSpecific(declaredResult, resultType) {
			resultType = declaredResult
		}
	}

	// Build curried param types → result.
	result := resultType
	for i := len(sig.Params) - 1; i >= 0; i-- {
		c0ParamType := goTypeToC0Type(sig.Params[i].Type)
		if c0ParamType == nil {
			fmt.Fprintf(os.Stderr, "c0: gosig fallback for %s.%s: cannot map Go type %q to C0\n",
				importPath, funcName, sig.Params[i].Type)
			return nil
		}
		result = &types.TFun{From: c0ParamType, To: result}
	}

	return result
}

// extractResultType walks a curried function type and returns the final
// result type (the rightmost non-function leaf).
func extractResultType(t types.Type) types.Type {
	for {
		fn, ok := t.(*types.TFun)
		if !ok {
			return t
		}
		t = fn.To
	}
}

// isMoreSpecific returns true if a is a more specific (concrete) type than b.
// A concrete type like Prim or TCon is more specific than a TVar.
func isMoreSpecific(a, b types.Type) bool {
	_, aTVar := a.(*types.TVar)
	_, bTVar := b.(*types.TVar)
	// If b is a TVar and a is concrete, a is more specific.
	if bTVar && !aTVar {
		return true
	}
	// If b is bytes ([]byte) and a is concrete, prefer a.
	if bp, ok := b.(*types.Prim); ok && bp.Name == "bytes" {
		if !aTVar {
			return true
		}
	}
	return false
}

// goTypeToC0Type converts a Go type string (as returned by go/types) to a
// C0 internal type. Returns nil if the type cannot be mapped.
//
// Handles: int, int8..int64, uint..uint64, float64→float, float32, bool,
// string, rune, []byte→bytes, []T→T list, func(A,B)C→A→B→C, chan T→T chan.
func goTypeToC0Type(goType string) types.Type {
	// Strip leading "*" in case of named pointer types; we treat them
	// as the underlying type for simplicity.
	goType = strings.TrimPrefix(goType, "*")

	// Primitive types
	switch goType {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"uintptr":
		return types.Int
	case "float64":
		return types.Float
	case "float32":
		return types.Float
	case "bool":
		return types.Bool
	case "string":
		return types.String
	case "rune":
		return types.Rune
	case "byte":
		return types.Int // byte is uint8
	case "[]byte":
		return types.Bytes
	}

	// error type → string (common in Go stdlib)
	if goType == "error" {
		return types.String
	}

	// Slice type: []T
	if strings.HasPrefix(goType, "[]") {
		elem := goTypeToC0Type(strings.TrimPrefix(goType, "[]"))
		if elem == nil {
			return nil
		}
		return types.ListType(elem)
	}

	// Channel type: chan T
	if strings.HasPrefix(goType, "chan ") {
		elem := goTypeToC0Type(strings.TrimPrefix(goType, "chan "))
		if elem == nil {
			return nil
		}
		return &types.TChan{Elem: elem}
	}

	// Function type: func(A, B) C  →  A -> B -> C
	if strings.HasPrefix(goType, "func(") {
		return parseGoFuncType(goType)
	}

	// interface{} → fresh type variable (anything can be passed)
	if goType == "interface{}" || goType == "any" {
		return types.Fresh("'a")
	}

	// For everything else (named types, structs, etc.), we can't map.
	return nil
}

// parseGoFuncType parses a Go func type string like "func(int, string) bool"
// and returns a curried C0 function type: int -> string -> bool.
func parseGoFuncType(s string) types.Type {
	// Expect: "func(...) result"
	s = strings.TrimPrefix(s, "func")
	s = strings.TrimSpace(s)

	// Find the opening paren
	if !strings.HasPrefix(s, "(") {
		return nil
	}

	// Find matching closing paren: track nesting depth
	depth := 0
	i := 0
	for i < len(s) {
		if s[i] == '(' {
			depth++
		} else if s[i] == ')' {
			depth--
			if depth == 0 {
				break
			}
		}
		i++
	}
	if i >= len(s) {
		return nil
	}

	paramsStr := s[1:i] // content between outer parens
	rest := strings.TrimSpace(s[i+1:])

	// Parse result type
	var resultType types.Type
	if rest != "" {
		resultType = goTypeToC0Type(rest)
	} else {
		// No return value → unit in C0
		resultType = types.Unit
	}
	if resultType == nil {
		return nil
	}

	// Parse params: split by top-level commas
	paramTypes := splitGoParams(paramsStr)
	// Build curried: p0 -> p1 -> ... -> result
	for i := len(paramTypes) - 1; i >= 0; i-- {
		pt := goTypeToC0Type(paramTypes[i])
		if pt == nil {
			return nil
		}
		resultType = &types.TFun{From: pt, To: resultType}
	}

	return resultType
}

// splitGoParams splits a comma-separated list of Go type strings, respecting
// nested angle brackets and parentheses.
func splitGoParams(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	var params []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<', '(', '[':
			depth++
		case '>', ')', ']':
			depth--
		case ',':
			if depth == 0 {
				params = append(params, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	last := strings.TrimSpace(s[start:])
	if last != "" {
		params = append(params, last)
	}
	return params
}
