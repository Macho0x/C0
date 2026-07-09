# Goop 0.7.0

## Formatter
- **`src/internal/fmt`**: `goop fmt` pretty-prints the parse tree without desugaring (preserves `guard`, `go (move ...)`, etc.)
- Offside-rule layout for `match`, `if`, and `let` chains

## Concurrency safety
- **LINEAR008**: channel-mediated race tracking when mutable values are sent on channels while still accessible in the spawning scope (`[check] concurrent`)
- **DEADLOCK001**: narrow static deadlock lint for circular send/recv on unbuffered channels (`[check] deadlock`, default `warn`)

## Examples
- `docs/examples/channel_race.goop`

## Tests
- Full `go test ./...` and 32 `goop test` integration tests passing
