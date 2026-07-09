# 3. Errors and effects

## Result type and `?`

`result` models success or failure. Use `?` inside `result { ... }` blocks to propagate errors:

```goop
let parseInt (s: string) : int result = ...

let compute (input: string) : int result =
  result {
    let! n = parseInt input;
    let! doubled = Ok (n + n);
    return doubled
  }
```

See [`result.goop`](../examples/result.goop) and [`computation.goop`](../examples/computation.goop).

## Effect rows

Functions can declare which **effects** they use: `io`, `async`, `panic`, and others tracked by the type checker.

```goop
let greet () : unit with { io } =
  print_line "hello"

let main () : unit with { io } =
  greet ()
```

A function annotated `with { }` (pure) cannot call `print_line` — the compiler reports **UNIFY018**:

```
function declared pure `with {}` but body uses effects: io
```

`goop.toml` enables body effect inference by default:

```toml
[check]
effect_inference = true
```

When inference is on, omitted `with` clauses are inferred from the body. Explicit `with { }` still means “must be pure.”

## Async computation expressions

```goop
let compute () : int chan =
  async {
    return 42
  }
```

See [`async.goop`](../examples/async.goop).

## Next

[4. Go interop →](04-go-interop.md)
