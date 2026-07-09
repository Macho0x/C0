# Goop Semantics

## Scoping

Goop uses lexical scoping. A `let` binding introduces a name in the body following it (or in the rest of the module for top-level bindings). Modules provide a namespace boundary. `open` imports exported names unqualified.

## Evaluation order

Function arguments are evaluated left-to-right before the function is applied. Record fields are evaluated left-to-right. `match` cases are tried in source order.

## Immutability

By default, all bindings and record fields are immutable. Mutation is explicit via `mutable` and the `<-` assignment operator. Mutable fields may be updated with `<-` as well.

```goop
let mutable x = 0
x <- x + 1

let r = { mutable value = 0 }
r.value <- r.value + 1
```

## Pattern matching semantics

A `match` expression evaluates the scrutinee once, then selects the first case whose pattern matches and whose guard (if any) is true. Pattern variables are bound in the corresponding branch.

Patterns are matched structurally:

- A constructor pattern matches values of that constructor.
- A record pattern matches records with the specified fields.
- A list pattern matches nil or cons cells.
- A tuple pattern matches tuples of the same arity.
- `_` matches any value and binds nothing.
- An identifier pattern matches any value and binds it.

The compiler guarantees exhaustiveness: every possible value of the scrutinee type must be covered by at least one pattern.

## Recursion

Recursive bindings must be marked with `rec`:

```goop
let rec length (xs: 'a list) : int =
  match xs with
  | [] -> 0
  | _ :: rest -> 1 + length rest
```

Mutually recursive bindings are joined with `and`:

```goop
let rec even (n: int) : bool =
  if n = 0 then true else odd (n - 1)
and odd (n: int) : bool =
  if n = 0 then false else even (n - 1)
```

## Equality

Structural equality is provided by the `=` and `<>` operators for types that support it. Equality is not defined for function types or abstract extern types unless the user provides it.

## Laziness

Goop is call-by-value. There is no built-in lazy evaluation in v1.

## Effects

Goop has row-polymorphic effect tracking in the type system. Effect rows are compile-time only and erased in Go output.

### Effect row syntax

Functions may declare their effects with a `with` clause after the return type:

```goop
let f (x: int) : int with { io } = ...
(* f may perform IO *)

let g (x: int) : int with {} = ...
(* g is explicitly pure *)

let h (x: int) : int = ...
(* h has unknown effects; backward compatible *)
```

### Effect row unification

Effect rows unify following the same algorithm as record rows:

- `with { io; log }` — closed: exactly `io` and `log`.
- `with { io | e }` — open (row-polymorphic): at least `io`, with any additional effects captured in the effect variable `e`.
- `with {}` — closed empty row: explicitly pure.
- No `with` clause — unknown effects (treated as `with { .. }` for unification).

Two closed rows unify if they have the same effects. An open row `with { a | e }` unifies with any row `with { a; b; ... }` that contains `a` (and possibly others). The variable `e` is instantiated with the remaining effects.

Extern Go functions default to unknown effects unless the user declares an explicit `with` clause.

### Erased at runtime

Effect rows impose zero runtime cost. They are validated at compile time and erased entirely in the Go output.

## Refinement contract evaluation

`where` clauses on types are runtime assertions. They do not affect compile-time type checking beyond the structure of the type itself.

### Parameter refinements (preconditions)

For `f (x: T where P) : U`, the compiler inserts a check at function entry:

```
if !P[x / it] then panic("f: precondition violated: P")
```

The identifier `it` in `P` refers to the parameter value `x`.

### Return refinements (postconditions)

For `f (...) : U where Q`, the compiler inserts a deferred check:

```
defer func() {
    if !Q[result / result] then panic("f: postcondition violated: Q")
}()
```

The identifier `result` in `Q` refers to the function's return value (lowered to a Go named return parameter).

### No compile-time verification

There is no SMT solver. Refinements are checked only at runtime. This is by design (see `docs/design/08-deferred-features-analysis.md`).

## Linear type discharge

Types declared with `: 1` are linear resource types. The compiler performs flow-sensitive discharge checking.

### Discharge rule

The conservative v1 rule is **first use = hand-off = discharge**. When a linear variable is used in any expression (passed as argument, captured in closure, used as scrutinee), it is considered handed off. The compiler tracks which linear variables are "live" at each program point.

### Errors

The compiler reports two classes of linearity errors:

1. **Double use**: using a linear variable that has already been discharged.
2. **Failure to discharge**: a live linear variable at the end of its scope.

### Erased at runtime

Linear types are erased in Go output. They lower to `interface{}` or the extern-declared Go type. Linearity is a purely compile-time discipline.

## Region scope semantics

`region { ... }` is a computation expression for scoped resource management.

### Operations

- `let! x = e` — acquires a linear resource from `e`, binds it to `x`, and registers cleanup (`defer Close(x)` in Go output). The variable `x` is auto-discharged at region exit.
- `do! e` — executes `e` for its effects (unit-typed).
- `let x = e` — binds a non-linear value.
- `return e` — produces the final result of the region.

### Auto-discharge

The linear discharge checker recognizes `let!`-bound variables as region-bound. These variables are automatically considered discharged at the region exit point — no explicit hand-off is required.

### Lowering

Regions lower to inline Go code. Each `let!` emits `defer Close(varName)`. There is no runtime region abstraction; the cleanup is inlined directly into the enclosing function.

## Panics

A panic aborts the current goroutine and unwinds the stack. It is intended for unrecoverable programmer errors, not expected runtime conditions.
