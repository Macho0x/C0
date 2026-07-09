# Language builtins

Builtins are part of the type system — not modules and not prelude bindings.

## Primitive types

| Type | Description |
|---|---|
| `int` | Machine integer |
| `float` | Floating point |
| `bool` | `true` / `false` |
| `string` | UTF-8 string |
| `unit` | Unit type `()` |
| `bytes` | Byte sequence |
| `rune` | Unicode code point |

## Lists

| Syntax | Meaning |
|---|---|
| `'a list` | Polymorphic list type |
| `[]` | Empty list |
| `x :: xs` | Cons |
| `[a; b; c]` | List literal |

Prelude: `list_length`, `list_append`. Higher-order: `std.list.Map`.

## Arrays

| Syntax | Meaning |
|---|---|
| `'a array` | OCaml-style dynamic array (lowers to Go slice) |
| `Array.make n default` | Allocate and initialize (prelude) |
| `Array.length arr` | Element count (prelude) |
| `arr.(i)` | Index read |
| `arr.(i) <- v` | In-place write |

Optional import-style access: [`std.array`](std-array.md).

## Option

| Constructor | Type |
|---|---|
| `None` | `'a option` |
| `Some x` | `'a option` |

Optional predicates: [`std.option`](std-option.md).

## Result

| Constructor | Type |
|---|---|
| `Ok x` | `('ok, 'err) result` |
| `Error e` | `('ok, 'err) result` |

Optional predicates: [`std.result`](std-result.md). Error propagation: `?` inside `result { ... }`.

## Channels

| Type | Created via |
|---|---|
| `'a chan` | `Chan.make` (prelude) |
| `'a owned_chan` | `OwnedChan.make` (prelude, linear) |

## Type-level features

- **Effect rows** — `with { io; async }` on function types (erased in Go output)
- **Refinements** — `where` clauses on parameters
- **Newtypes** — `type t = newtype string` with constructor `T "value"`
- **Linear types** — `type handle : 1` for quantity-1 resources

See [type system](../design/02-type-system.md) and [syntax](../design/03-syntax.md).
