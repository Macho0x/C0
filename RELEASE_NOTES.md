# Goop 1.7.0

Goop 1.7.0 completes the Treelog-driven FFI surface: string escapes and
slicing, heap `ptr_of` for interface implementors, slog Value/Kind/Level
interop, and full Go struct literals (including empty `Mutex`/`Buffer` and
`HandlerOptions`).

## Highlights

- **String escapes:** `"\x1b[31m"`, `"\033[0m"`, `"\e[0m"`.
- **`String.length` / `String.sub`:** Go byte length and slicing.
- **`ptr_of` handlers:** `slog.New (ptr_of { last = "" })` with no `@[go]` ctor.
- **Go struct FFI:** `ptr_of { level = LevelInfo }` for `HandlerOptions`;
  `ptr_of {}` for `sync.Mutex` / `bytes.Buffer`.
- **slog introspection:** `Value.Kind`, `Group`, `Record.Attrs` / fields;
  native `Level` comparisons.

## Treelog

Construction embeds for handlers, mutex, buffer, and JSON options can move to
native Goop. Residual `@[go]`: crypto UUID and Go-facing `Scope.Error`.

## Verification

- `go test ./...` (from `src/`) — all packages green
- `goop test tests/` → 101 passed, 0 failed
- Treelog: `goop test examples/` → 6 passed, 0 failed; `go build ./...`
