# Goop Semantics

## Scoping

Goop uses lexical scoping. A `let` binding introduces a name in the body following it (or in the rest of the module for top-level bindings). Modules provide a namespace boundary. Dot imports (`import goop . "path"`) bring exported names unqualified.

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

Array cells are mutable without marking the array binding `mutable`: `arr.(i) <- v` updates element `i` in place. The array variable itself remains immutable (you cannot rebind `arr` without `let mutable arr = ...`).

## Arrays

Goop provides OCaml-style dynamic arrays (`'a array`), lowered to Go slices.

| Operation | Syntax | Type |
|---|---|---|
| Create | `Array.make n default` | `int -> 'a -> 'a array` |
| Length | `Array.length arr` | `'a array -> int` |
| Read | `arr.(i)` | `'a array -> int -> 'a` |
| Write | `arr.(i) <- v` | `'a array -> int -> 'a -> unit` |

`Array.make n default` allocates `n` slots initialized to `default`. Index `i` must have type `int`; the compiler unifies the indexed value with the array element type.

Const-size array literals (`[| ... |]`) are **not** part of v0.8.0 — use `Array.make` plus a fill loop.

## For loops

`for var = from to to do body done` is an expression of type `unit`. The loop variable is bound to `int` in `body`. Bounds `from` and `to` must be `int`; iteration is inclusive on both ends (`from <= i <= to`).

```goop
for i = 0 to n - 1 do
  arr.(i) <- f i
done
```

The loop variable is not visible outside the loop.

## Sequencing with `begin` / `end`

`begin e1; e2; ... en end` evaluates expressions left-to-right. The value of the whole expression is the value of the last expression (`en`). Earlier expressions must be valid in sequence (typically `unit`, assignments, or loops).

Use `begin ... end` when you need statement-like sequencing inside a `let` body or expression position:

```goop
let result =
  begin
    arr.(0) <- 1;
    arr.(1) <- 2;
    arr.(0) + arr.(1)
  end
```

## Qualified constructors

Constructors may be written qualified by their type name: `Color.Red`, `OrderAck.PartialFill`. In `match` patterns, qualification disambiguates constructors when multiple types export the same name.

The qualified form does not change runtime representation — it is the same constructor as the unqualified name when unambiguous.

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

`where` clauses on types are checked at compile time by a built-in refinement solver when possible (REFINE001–003). Proven call sites skip runtime guards in generated Go; unproven call sites emit guards; disproven sites are compile errors. Exported functions always retain entry guards for FFI safety.

There is no external SMT solver (Z3 deferred). Simple integer arithmetic VCs are handled by interval analysis.

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
