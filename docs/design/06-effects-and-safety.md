# C0 Effects and Safety

## Safety defaults

C0 is designed to be safe by default:

- No null values. Absence is represented by `'a option`.
- No unchecked exceptions. Errors are returned via `result`.
- Exhaustive pattern matching.
- Immutable bindings by default; mutation is explicit.
- Bound-checked sequence access.

## Error handling

C0 uses `result` for recoverable errors and panics for programmer errors.

```c0
type ('ok, 'err) result = Ok of 'ok | Error of 'err
```

The `?` operator makes propagation ergonomic:

```c0
let readUser (id: int) : result<user, error> =
  let user = fetchUser id ?
  let validated = validate user ?
  Ok validated
```

## Panics

Panics represent bugs, not expected failures. They lower to Go `panic`. A C0 function that may panic is marked in documentation but is not tracked in the type system (matching Go's model).

```c0
let unsafeHead (xs: 'a list) : 'a =
  match xs with
  | x :: _ -> x
  | [] -> panic "empty list"
```

## Effects

C0 has row-polymorphic effect tracking at the type level. Effects are compile-time only and erased in Go output (zero runtime cost).

### Effect rows

Functions declare their effects with a `with` clause after the return type:

```c0
(* IO effect *)
let readFile (path: string) : string with { io } = ...

(* Pure function *)
let double (x: int) : int = x * 2

(* Row-polymorphic: at least log, plus any others *)
let withLog (f: unit -> 'a with { log | e }) : 'a with { e } = ...

(* Explicitly pure: no effects *)
let add (x: int) (y: int) : int with {} = x + y
```

### Effect names

Built-in and user-defined effect names include: `io`, `log`, `state`, `async`, `panic`, and any user-declared effect. Extern Go functions default to unknown effects unless an explicit `with` clause is provided.

### Row-polymorphic unification

Effect rows unify like record rows:

- `with { io; log }` — closed: exactly those two effects.
- `with { io | e }` — open: at least `io` plus any others captured in `e`.
- `with {}` — closed empty: explicitly pure.
- No `with` clause — unknown effects (backward compatible).

### Design choice: erased, not resumptive

C0's effect system is **tracking only** — there are no resumptive effect handlers. This is an intentional design choice. Resumptive effects require continuations or CPS, which violate C0's constraint of emitting idiomatic Go. See `docs/design/08-deferred-features-analysis.md` for the full analysis.

The legacy `[@io]`, `[@pure]`, and `[@panic]` attributes are superseded by effect rows. Existing code without `with` clauses continues to compile (backward compatible).

## Mutability

Bindings are immutable by default. Mutable bindings are explicit:

```c0
let x = 1            (* immutable *)
let mutable y = 1    (* mutable *)
y <- y + 1
```

Mutable fields in records are also explicit:

```c0
type counter = { mutable value: int }
```

## Resource safety

C0 provides two complementary mechanisms for resource safety:

### Linear resource types

Types declared with `: 1` are linear resources that must be discharged on every control-flow path:

```c0
type handle : 1

(* Close discharges a handle *)
let Close (h: handle) : unit = ...

(* h is discharged via hand-off to Close *)
let process (h: handle) : unit =
  Close h
```

The compiler performs flow-sensitive discharge checking. The conservative v1 rule is "first use = hand-off = discharge": passing a linear value to any function or expression discharges it. The compiler rejects double-use and failure-to-discharge.

Linear types are erased in Go output (lower to `interface{}`). The linearity discipline is compile-time only.

See also: `docs/examples/linear.c0`.

### Region scopes

`region { ... }` is a computation expression for scoped resource management with guaranteed cleanup:

```c0
let process (h: handle) : unit =
  region {
    let! x = h              (* acquires handle, inserts defer Close(x) *)
    do! useHandle x         (* uses the resource *)
    return ()
  }
```

Each `let!` binding acquires a linear resource and registers `defer Close(varName)` at region exit. The linear discharge checker auto-discharges region-bound variables.

Region scopes replace the legacy `using` block with compile-time-guaranteed cleanup.

See also: `docs/examples/region.c0`.

### Garbage collector

For non-linear types, Go's garbage collector handles memory management automatically. Resource cleanup for files, sockets, and handles relies on the mechanisms above.

## Concurrency

C0 exposes Go's concurrency model directly:

- Goroutines via `go` expressions.
- Channels via the `chan` type.
- `select` for channel multiplexing.
- `OwnedChan` — an opt-in linear (`: 1`) channel wrapper for single-producer patterns with compile-time close safety (send-after-close and double-close are caught at compile time as linear double-use errors). The default `chan` remains shared (multi-producer) with runtime safety checks.

```c0
let worker (ch: int chan) : unit =
  go (fun () ->
    let x = Chan.recv ch in
    Console.print_line (Int.to_string x))

(* Single-producer pattern with compile-time close safety *)
let producer () : unit =
  let ch : int owned_chan = OwnedChan.make () in
  OwnedChan.send ch 1
  OwnedChan.send ch 2
  OwnedChan.close ch
  (* OwnedChan.send ch 3  — compile error: ch already discharged *)
```

Async/await syntax is not part of v1.

## What is intentionally absent

- **Exceptions** — use `result`.
- **Null** — use `option`.
- **Unsafe casts** — use explicit conversions or `extern`.
- **Resumptive effect handlers** — row-polymorphic effect tracking is implemented; resumptive handlers are rejected (see `docs/design/08-deferred-features-analysis.md`).
- **Full ownership/lifetimes (Rust style)** — opt-in linear resource types (`: 1`) and region scopes are implemented; full borrowing is deferred.
