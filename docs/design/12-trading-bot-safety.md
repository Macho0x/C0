# Trading bot safety matrix (Goop v0.8.2)

Goop targets compile-time prevention of common trading-bot failure modes.

## Bug class → error code

| Bug class | Code | When |
|-----------|------|------|
| Missing venue ack variant | EXHAUST003 | Non-exhaustive `match` on ADT |
| Dead match arm / wildcard | EXHAUST001/002 | Unreachable patterns |
| Nil channel before init | NIL001 | `Chan.send`/`recv` before `Chan.make` |
| Shared mutable position race | LINEAR006/007 | Linear + goroutine analysis |
| Channel-mediated mutable race | LINEAR008 | `Chan.send` of mutable var still in parent scope |
| Potential channel deadlock | DEADLOCK001 | Circular send/recv between two goroutines |
| Send after close | LINEAR002 | Owned channel linearity |
| Leaked owned channel | LINEAR001 | Channel not discharged |
| Refinement violated at call | REFINE001 | VC disproves precondition |
| Unproven refinement | REFINE002 | Runtime guard emitted |
| Pure fn calls IO | UNIFY018 | `with {}` vs inferred `{ io }` |
| Branded ID mix-up | UNIFY* | `order_id` vs `symbol` newtypes |

## Compile-time vs runtime

- **Compile-time:** exhaustiveness, nil-channel (conservative), linearity, effect rows, newtypes, proven refinements, narrow channel deadlock lint (DEADLOCK001).
- **Runtime:** unproven refinements (REFINE002 guards), external I/O failures, venue API semantics, total deadlocks (Go runtime).

## Recommended patterns

1. Model venue messages as ADTs; handle with exhaustive `match`.
2. Use `type order_id = newtype string` for symbols, order IDs, client IDs.
3. Pass position state through `owned_chan` or single-owner linear types.
4. Annotate `main` and IO helpers with `with { io }` / `with { async }`.
5. Initialize feeds with `let ch = Chan.make ()` before any send/recv.

See `docs/examples/trading_*.goop` and `orderbook.goop` for worked examples.
