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

```goop
let safeDiv (a: int) (b: int where b <> 0) : int =
  a / b
```

Unproven refinements get runtime checks. See [`contracts.goop`](../examples/contracts.goop).

## Error code reference

- [Trading bot safety matrix](../design/12-trading-bot-safety.md)
- [Full error catalog](../design/10-error-reference.md)

## Next steps

- [Standard library reference](../stdlib/README.md)
- [All examples](../examples/)
- [Contributing](../../CONTRIBUTING.md)
