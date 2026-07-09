# 6. Safety checks

Goop runs a unified safety pipeline on every `goop check` / `goop build` / LSP diagnostic.

## Exhaustive matching (EXHAUST003)

Non-exhaustive `match` is a **compile error** by default:

```
✕ EXHAUST003: non-exhaustive match: missing constructor(s): PartialFill
```

Example: [`trading_order.goop`](../examples/trading_order.goop) — all `OrderAck` variants handled.

## Nominal newtypes

Brand primitive types so they cannot be confused:

```goop
type order_id = newtype string
type symbol = newtype string

let oid = OrderId "ord-1"
let sym = Symbol "ETH-USD"
```

Raw strings cannot be assigned to `order_id` without the constructor. See [`newtype_trading.goop`](../examples/newtype_trading.goop).

## Nil channel detection (NIL001)

Using a channel before initialization is rejected. See [`channel_safety.goop`](../examples/channel_safety.goop).

## Refinement contracts

The refine pass proves simple arithmetic VCs at compile time. **Proven** call sites skip runtime guards; **unproven** sites emit call-site checks in generated Go; **disproven** sites are compile errors (REFINE001).

```goop
let safeDiv (a: int) (b: int where b <> 0) : int = a / b

let compute (x: int) (y: int) : int =
  if y <> 0 then safeDiv x y else 0   (* proven — no call-site guard *)
```

See [`refinement_solving.goop`](../examples/refinement_solving.goop) and [`contracts.goop`](../examples/contracts.goop).

Configure unproven severity:

```toml
[check]
refinement_unproven = "warn"   # warn | error | off
```

## `goop.toml` check severities

| Key | Default | Controls |
|---|---|---|
| `exhaust_redundant` | `warn` | EXHAUST001/002 |
| `exhaust_missing` | `error` | EXHAUST003 |
| `effect_inference` | `true` | Infer `with` from bodies |
| `concurrent` | `error` | LINEAR006/007/008 race warnings |
| `refinement_unproven` | `warn` | REFINE002 unproven VCs |
| `deadlock` | `warn` | DEADLOCK001 channel deadlock lint |

## Error code reference

- [Trading bot safety matrix](../design/12-trading-bot-safety.md)
- [Full error catalog](../design/10-error-reference.md)

## Next steps

- [Standard library reference](../stdlib/README.md)
- [All examples](../examples/)
- [Contributing](../../CONTRIBUTING.md)
