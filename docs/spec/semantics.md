# Goop Semantics

## Scoping

Goop uses lexical scoping. A `let` binding introduces a name in the body following it (or in the rest of the module for top-level bindings). Modules provide a namespace boundary. Dot imports (`import goop . "path"`) and `open` bring exported names unqualified.

## Evaluation order

Function arguments are evaluated left-to-right before the function is applied. Record fields are evaluated left-to-right. `match` cases are tried in source order.

## Immutability and refs

By default, bindings and record fields are immutable. Mutation uses:

- `ref` cells: `let r = ref e in …`, read `!r`, write `r := e`
- `mutable` record fields: `r.value <- …`
- Array cells: `arr.(i) <- v` (the array binding itself stays immutable)

There is no `let mutable` local binding.

```goop
let r = ref 0 in
r := !r + 1

let rec = { mutable value = 0 } in
rec.value <- rec.value + 1
```

## Arrays

| Operation | Syntax | Type |
|---|---|---|
| Create | `Array.make n default` | `int -> 'a -> 'a array` |
| Length | `Array.length arr` | `'a array -> int` |
| Read | `arr.(i)` | `'a array -> int -> 'a` |
| Write | `arr.(i) <- v` | `'a array -> int -> 'a -> unit` |

Const array literals `[| … |]` are supported in 1.0.

## Loops

`for var = from to to do body done` — type `unit`, inclusive bounds, loop variable `int` scoped to the body.

`while cond do body done` — type `unit`; evaluates `cond` before each iteration.

## Sequencing with `begin` / `end`

`begin e1; e2; ... en end` evaluates left-to-right; the value is `en`.

## Qualified constructors

`Color.Red` is the same constructor as `Red` when unambiguous; qualification disambiguates.

## Pattern matching semantics

Scrutinee evaluated once; first matching arm with true `when` guard wins. Exhaustiveness is required.

## Exceptions

```goop
exception Fail of string

try e with | Fail msg -> handle msg
try e finally cleanup
raise (Fail "x")
failwith "msg"
```

`raise` / `failwith` abort the current evaluation (lower to Go `panic`). `try/with` may recover. Prefer `result` for expected failures.

## Recursion

`let rec` / `and` for recursive and mutually recursive bindings.

## Equality

`=` and `<>` for structural equality on supported types. Not defined for function types unless provided.

## Laziness

Call-by-value. `lazy e` is supported as a delayed thunk (minimal).

## Effects

OCaml 5-style `effect` / `perform` / handlers. Effectful evaluation may be CPS-transformed. Surface effect rows (`with { io }`) are not part of the 1.0 language.

## Refinement contract evaluation

`where` clauses generate VCs. The built-in solver (and optional Z3) may prove them at compile time. Proven call sites skip runtime guards; unproven sites emit guards; disproven sites are errors. Exported functions retain entry guards.

## Linear type discharge

`: 1` types: first use = hand-off = discharge. Double-use and failure-to-discharge are errors. Erased at runtime.

## Resource cleanup

`try … finally` runs cleanup after the body (including on panic unwind where Go `defer` applies). No `region { }` CE semantics.
