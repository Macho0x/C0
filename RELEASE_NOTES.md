# Goop 1.4.0

Goop 1.4.0 adds Go method and field imports. Goop code can now call selectors
on opaque imported Go values without introducing a one-off `@[go]` adapter.

## Highlights

- **Method imports:** declare `val (x : T).M : A -> B` in an `import go`
  block and call it with `x.M arg` (or `T.M x arg`).
- **Field imports:** a non-arrow selector type, such as
  `val (a : Attr).Key : string`, lowers to a Go field read.
- **Go-shaped values:** callbacks, `go_slice` indexing (`xs.(i)`), `any_of`,
  and `spread` work with imported methods and variadic APIs.
- **Examples:** [`go_method_calls.goop`](docs/examples/go_method_calls.goop)
  imports `bytes.Buffer.String`; the native
  [`slog.Handler`](docs/examples/go_implements_slog_handler.goop) example
  now imports and calls `slog.New` and `Logger.Info`.

## Verification

- `goop check docs/examples/go_method_calls.goop`
- `goop check docs/examples/go_implements_stringer.goop`
- `goop check docs/examples/go_implements_slog_handler.goop`
- `goop test tests/`
- Verify generated Go contains direct selector calls such as `b.String()` and
  accepts the emitted `slog.Handler` assertion.
