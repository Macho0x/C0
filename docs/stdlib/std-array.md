# std-array / Array builtins

Goop provides OCaml-style arrays via prelude bindings (no import required).

## Types

```goop
int array          (* int array *)
decision array     (* user type T array *)
```

## Functions

| Name | Type | Go lowering |
|------|------|-------------|
| `Array.make` | `int -> 'a -> 'a array` | `make([]T, n)` + fill loop |
| `Array.length` | `'a array -> int` | `len(arr)` |

## Operators

| Syntax | Meaning |
|--------|---------|
| `arr.(i)` | Index read |
| `arr.(i) <- v` | In-place write (requires mutable cell or `let mutable`) |

## Example

```goop
let lut = Array.make 100 default in
for i = 0 to 99 do
  lut.(i) <- compute i
done
```

See [trading_decision_lut.goop](../examples/trading_decision_lut.goop).
