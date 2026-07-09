# 5. Concurrency

Goop exposes Go’s concurrency primitives with compile-time safety checks.

## Channels and goroutines

Prelude bindings (no import required):

| Binding | Type | Effect |
|---|---|---|
| `Chan.make` | `unit -> 'a chan` | pure |
| `Chan.send` | `'a chan -> 'a -> unit` | `async` |
| `Chan.recv` | `'a chan -> 'a` | `async` |
| `Chan.close` | `'a chan -> unit` | `async` |

```goop
let main () : unit with { async } =
  let ch = Chan.make () in
  let ignored = go (fun () -> Chan.send ch 42) in
  let v = Chan.recv ch in
  assert (v = 42)
```

See [`concurrency.goop`](../examples/concurrency.goop).

## Effect annotations for async

`go` and channel operations require `async` in the effect row:

```goop
let worker () : unit with { async } = ...
let main () : unit with { async } = worker ()
```

Using only `with { io }` when the body spawns goroutines yields **UNIFY019**.

## Data race detection

The linear checker flags `mutable` variables captured by `go` while still accessible in the spawning scope:

```goop
let race () : unit with { async; io } =
  let mutable counter = 0 in
  let ignored = go (fun () -> print_line (int_to_string counter)) in
  print_line (int_to_string counter)   (* error: counter still in scope *)
```

Good patterns (capture without post-`go` use): [`race_detection.goop`](../examples/race_detection.goop).

## `go (move ...)`

When a `mutable` binding is transferred into a goroutine and the parent no longer uses it, suppress race warnings explicitly:

```goop
let launch () : unit with { async; io } =
  let mutable counter = 0 in
  let dummy = go (move counter) (fun () -> print_line (int_to_string counter)) in
  ()
```

See [`go_move.goop`](../examples/go_move.goop).

## Linear `go` handoff

Linear resources (`owned_chan`, `type handle : 1`) can be handed off to a goroutine — the parent scope discharges the variable. See [`linear_go_handoff.goop`](../examples/linear_go_handoff.goop).

Configure race severity in `goop.toml`:

```toml
[check]
concurrent = "error"   # warn | error | off
```

## Owned channels

Linear `owned_chan` types enforce close safety. See [`owned_chan.goop`](../examples/owned_chan.goop) and [`linear.goop`](../examples/linear.goop).

## Next

[6. Safety checks →](06-safety-checks.md)
