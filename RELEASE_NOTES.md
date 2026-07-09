# Goop 0.6.0

## Compile-time safety completion

- **Refinement call-site guards**: proven VCs skip runtime checks; unproven calls emit guards (IIFE-wrapped in expression position); exported functions keep entry guards for FFI safety.
- **Arithmetic refinement solver**: interval arithmetic with overflow-safe bounds; structural implication (`a > b` → `a - b > 0`).
- **Linear `go` handoff**: linear/`owned_chan` resources captured by `go` are discharged in the spawning scope.
- **`go (move x, ...)` syntax**: explicit transfer of mutable bindings into goroutines; suppresses LINEAR007 when the parent no longer uses the variable.
- **`goop.toml` severities**: `concurrent` (LINEAR006/007) and `refinement_unproven` (REFINE002) support `warn` | `error` | `off`.

## Examples

- `docs/examples/refinement_solving.goop` (extended)
- `docs/examples/go_move.goop`
- `docs/examples/linear_go_handoff.goop`

## Tests

- 31 `*_test.goop` integration tests passing
- Full `go test ./...` green
