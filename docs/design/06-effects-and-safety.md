# Goop Effects and Safety

## Safety defaults

- No null values — use `'a option`.
- Recoverable errors — `('ok, 'err) result` + `match`.
- Bugs / aborts — `failwith` / `raise` (not a `panic` keyword).
- Exhaustive pattern matching.
- Immutable bindings by default; mutation via `ref` / `!` / `:=` or `mutable` fields.
- Bound-checked sequence access where applicable.

## Error handling

```goop
type ('ok, 'err) result = Ok of 'ok | Error of 'err

let readUser (id: int) : (user, string) result =
  match fetchUser id with
  | Error e -> Error e
  | Ok user ->
      match validate user with
      | Error e -> Error e
      | Ok validated -> Ok validated
```

There is no `?` propagation operator (PARSE-MIG012). Prefer plain `match`, or `let*` / `let+` where supported.

## Failwith and exceptions

```goop
exception Boom
exception Fail of string

let unsafeHead (xs: 'a list) : 'a =
  match xs with
  | x :: _ -> x
  | [] -> failwith "empty list"

let run () =
  try
    raise (Fail "oops")
  with
  | Fail msg -> msg
  | Boom -> "boom"
```

`failwith` / `raise` lower to Go `panic`. `try/with` lowers via `recover`. Use exceptions for bugs and unexpected failure; use `result` for expected venue/trading errors.

## Effect handlers (OCaml 5-style)

Goop 1.0 ships minimal OCaml 5 effect handlers:

```goop
effect Flip : unit -> bool
```

- `perform` invokes an effect operation.
- Handlers appear as `effect (Op …) k -> …` arms in `match` / `try`.
- Effectful code is CPS-transformed (`internal/effects`) before Go codegen — **not** idiomatic Go.
- Pure code without `perform` stays direct-style.

Surface effect **rows** (`… with { io }`) are removed (PARSE-MIG016). See [14-ocaml-parity.md](14-ocaml-parity.md) and [08-deferred-features-analysis.md](08-deferred-features-analysis.md).

## Mutability

```goop
let x = 1              (* immutable *)
let r = ref 1 in       (* ref cell *)
r := !r + 1

type counter = { mutable value: int }
```

No `let mutable` locals.

## Resource safety

### Linear resource types

```goop
type handle : 1
let Close (h: handle) : unit = ...
let process (h: handle) : unit = Close h
```

First use = discharge. Erased in Go.

### Cleanup

Prefer `try … finally` or explicit close. `region { }` / `using` computation expressions are removed.

### Garbage collector

Non-linear values use Go’s GC.

## Concurrency

- `go` expressions, `chan`, `select`
- `owned_chan` — linear single-producer close safety
- `go (move r)` for transferring a `ref` (or other binding) into a goroutine

```goop
let worker (ch: int chan) : unit =
  go (fun () ->
    let x = Chan.recv ch in
    print_line (int_to_string x))

let launch () : unit =
  let r = ref 0 in
  let _ = go (move r) (fun () -> r := !r + 1) in
  ()
```

## What is intentionally absent

- **Null** — use `option`.
- **Unsafe casts** — use explicit conversions or `@golang`.
- **Full ownership/lifetimes** — linear `: 1` only.
- **F# computation expressions / Kit macros / Dingo `?`** — removed in 1.0.
