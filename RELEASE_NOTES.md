# Goop 1.3.0

Goop 1.3.0 adds native Go interface implementations, allowing Goop record
types to satisfy interfaces such as `fmt.Stringer` and `slog.Handler` without
writing an `@[go]` wrapper for their methods.

## Highlights

- **`implements` declarations:** `implements Interface for Type with … end`
  generates pointer-receiver Go methods and a compile-time interface
  assertion.
- **Go type imports:** use `type Name` inside an `import go` signature block
  to import opaque Go named types and interfaces.
- **FFI value types:** `error`, `'a ptr` (`ptr_of`, `null`, `is_null`), and
  `'a go_slice` plus length, append, and list-conversion helpers.
- **Examples:** native
  [`fmt.Stringer`](docs/examples/go_implements_stringer.goop) and
  [`slog.Handler`](docs/examples/go_implements_slog_handler.goop)
  implementations.

## Verification

- `goop check docs/examples/go_implements_stringer.goop`
- `goop check docs/examples/go_implements_slog_handler.goop`
- `goop test tests/`
- Verify generated Go accepts the emitted `fmt.Stringer` and `slog.Handler`
  assertions during the example builds.
