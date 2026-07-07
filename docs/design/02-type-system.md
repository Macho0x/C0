# C0 Type System

## Design goals

C0's type system aims to catch common errors at compile time while remaining implementable as a source-to-source compiler to Go. The priorities are:

1. Sound, global type inference (Hindley-Milner style).
2. Null safety through explicit `option`.
3. Exhaustive pattern matching over algebraic data types.
4. Explicit, typed error handling through `result`.
5. Parametric polymorphism without higher-kinded types.

## Core types

### Primitive types

| C0 type | Lowered Go type | Notes |
|---|---|---|
| `int` | `int` | Platform word size |
| `int8`, `int16`, `int32`, `int64` | `int8`, `int16`, `int32`, `int64` | Signed fixed-width |
| `uint`, `uint8`, `uint16`, `uint32`, `uint64` | `uint`, `uint8`, `uint16`, `uint32`, `uint64` | Unsigned fixed-width |
| `float` | `float64` | Default float |
| `float32` | `float32` | |
| `bool` | `bool` | |
| `string` | `string` | Immutable UTF-8 |
| `rune` | `rune` | Unicode code point |
| `bytes` | `[]byte` | Mutable byte slice |
| `unit` | `struct{}` | The `()` value |

### Type variables

Polymorphic functions use quoted type variables:

```c0
let identity (x: 'a) : 'a = x
let pair (a: 'a) (b: 'b) : 'a * 'b = (a, b)
```

### Tuples

```c0
let p : int * string = (42, "hello")
let (x, y) = p
```

Lowered to Go structs with positional fields when needed.

## Algebraic data types

ADTs are the primary abstraction for sums:

```c0
type color = Red | Green | Blue

type shape =
  | Circle of { radius: float }
  | Rect of { width: float; height: float }
  | Point
```

ADT values must be constructed with their constructor and deconstructed with `match`. There are no implicit conversions and no null variants.

### Record types

Records are product types with named fields. They are **closed by default**: a record value must have exactly the fields declared.

```c0
type point = { x: float; y: float }

let origin : point = { x = 0.0; y = 0.0 }
let moved = { origin with x = 1.0 }
```

### Row polymorphism (optional, v1.1)

Functions may accept any record with at least a given set of fields:

```c0
let print_name (r: { name: string | .. }) : unit =
  Console.print_line r.name
```

This is opt-in via the `| ..` annotation. Closed records lower to Go structs; open records lower to Go interfaces or explicit accessor functions.

## `option` and `result`

These are built-in generic ADTs:

```c0
type 'a option = None | Some of 'a
type ('ok, 'err) result = Ok of 'ok | Error of 'err
```

There is no `null` in C0. A value of type `string` is always a string. Absence is represented as `string option`.

## Pattern matching

`match` is exhaustive. The compiler rejects matches that omit variants:

```c0
let describe (c: color) : string =
  match c with
  | Red -> "red"
  | Green -> "green"
  | Blue -> "blue"
```

Patterns may be nested, include guards, and use active patterns.

## Active patterns

Active patterns let users define new patterns as functions:

```c0
let (|Positive|_|) (n: int) : int option =
  if n > 0 then Some n else None

match x with
| Positive p -> print p
| _ -> ()
```

## Generics

C0 supports parametric polymorphism but **not** higher-kinded types. This matches Go's generic capabilities and keeps lowering straightforward.

```c0
let map (f: 'a -> 'b) (xs: 'a list) : 'b list =
  match xs with
  | [] -> []
  | x :: rest -> f x :: map f rest
```

## Effect rows

C0 has row-polymorphic effect tracking in the type system. Effect rows are compile-time only (erased in Go output) and track which side effects a function may perform.

### Syntax

Effect rows appear after a function return type using `with`:

```c0
(* Explicit IO effect *)
let readFile (path: string) : string with { io } = ...

(* Pure function: no effects *)
let double (x: int) : int = x * 2

(* Row-polymorphic: works with any effect set *)
let catchAll (f: unit -> 'a with { e }) : 'a with { e } = ...

(* Explicitly pure *)
let add (x: int) (y: int) : int with {} = x + y
```

### Effect names

Effects are simple identifiers: `io`, `log`, `state`, `async`, `panic`, etc. User code and extern functions may introduce new effect names.

### Row-polymorphic unification

Effect rows unify exactly like record rows (see "Row polymorphism" above):

- `with { io; log }` — closed: exactly those two effects.
- `with { io | e }` — open: at least `io` plus any others captured in `e`.
- `with {}` — closed empty row: explicitly pure.
- No `with` clause — unknown effects (backward compatible, equivalent to `with { .. }` for inference).

The compiler unifies effect rows during type checking. An open row unifies with any row that contains at least the specified effects. Two closed rows must match exactly.

### Erased at runtime

Effect rows are completely erased in the Go output. They impose zero runtime cost. The type checker validates effect usage and reports errors at compile time.

Extern Go functions default to unknown effects unless the user declares an explicit `with` clause.

See also: `docs/examples/effects.c0`, `docs/design/08-deferred-features-analysis.md`.

## Refinement contracts

C0 supports lightweight runtime refinement contracts via `where` clauses on types. These are runtime assertions (no SMT solver) that `panic` on violation.

### Syntax

```c0
(* Parameter refinement: `it` refers to the parameter value *)
let safeDiv (a: int) (b: int where b <> 0) : int = a / b

(* Return refinement: `result` refers to the return value *)
let clamp (x: int) (lo: int) (hi: int where hi >= lo) : int where result >= lo && result <= hi =
  if x < lo then lo
  else if x > hi then hi
  else x
```

`where` is a postfix type modifier. `it` refers to the value being constrained in parameter refinements; `result` refers to the return value in return type refinements.

### Semantics

- **Preconditions** (`where` on a parameter type): lowered to `if !(pred) { panic("funcName: precondition violated: pred") }` at function entry.
- **Postconditions** (`where` on the return type): lowered to `defer func() { if !(pred) { panic("funcName: postcondition violated: pred") } }()` using Go named return values.

There is **no SMT solver** and no compile-time proof obligation. Refinements are checked only at runtime. This is an intentional design choice (see `docs/design/08-deferred-features-analysis.md` for the full analysis of why SMT-based refinement types are deferred).

See also: `docs/examples/contracts.c0`.

## Linear resource types

C0 supports opt-in modal linearity. A type declared with `: 1` is a linear resource type that must be discharged (used/handed-off) on every control-flow path.

### Syntax

```c0
(* Declare a linear resource type *)
type handle : 1
```

Types without the `: 1` annotation default to unrestricted (`ω`), which is the usual unrestricted structural type.

### Discharge checking

The compiler performs flow-sensitive discharge checking via the `internal/linear` package. It tracks which linear variables are "live" at each program point and reports errors for:

- **Double use**: using a linear variable after it has already been handed off.
- **Failure to discharge**: a linear variable is still live at the end of its scope.

The conservative v1 rule is: **first use = hand-off = discharge**. When a linear value is passed as a function argument, captured in a closure, or used in any expression, it is considered handed off and no longer available.

### Erased at runtime

Linear types are erased in Go output. They lower to `interface{}` or the extern-declared Go type. The linearity discipline is enforced purely at compile time.

See also: `docs/examples/linear.c0`, `docs/design/08-deferred-features-analysis.md`.

## What is intentionally absent

- **Null.** Use `option`.
- **Implicit conversions.** All numeric coercions are explicit.
- **Exceptions.** Use `result`.
- **Higher-kinded types.** Go cannot express them.
- **Full dependent types (Idris/Agda style).** Lightweight runtime refinement contracts (`where` clauses) are implemented; full SMT-based dependent/refinement types are deferred (see `docs/design/08-deferred-features-analysis.md`).
- **Borrow checker with lifetimes (Rust style).** Opt-in linear resource types (`: 1`) with discharge checking are implemented; full ownership and borrowing are deferred.
- **Resumptive effect system.** Erased row-polymorphic effect tracking is implemented; resumptive handlers are rejected (see `docs/design/08-deferred-features-analysis.md`).
- **Implicit side effects.** Functions are pure unless their type or body says otherwise.

## Type inference

C0 uses Hindley-Milner style inference with let-polymorphism. Top-level declarations may omit types; local bindings are generalized at `let`. Function parameters generally require annotations at module boundaries and for exported functions.

The bootstrap compiler uses a layered approach:

1. Local constraint solving for monomorphic expressions.
2. Unification-based polymorphism for `let` bindings, including effect row unification.
3. Optional `gopls` fallback for higher-order functions whose types cannot be inferred locally.

Effect variables are fresh type variables constrained by row unification — exactly how `'a` is constrained for record rows. When a function has no `with` clause, its effects are inferred from its body. When an explicit `with` clause is provided, the inferred effects must be compatible with the declared row.
