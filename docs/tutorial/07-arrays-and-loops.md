# 7. Arrays and loops

Goop uses **OCaml-style** arrays and imperative loops for LUTs, buffers, and in-place updates. This is the canonical surface in v0.8.0 — not Rust/Go bracket syntax.

## Dynamic arrays

Type: `'a array` (postfix, like OCaml `int array`).

```goop
let arr = Array.make 10 0 in
assert (Array.length arr = 10 && arr.(0) = 0)
```

| Syntax | Meaning |
|---|---|
| `Array.make n default` | Allocate `n` slots, each initialized to `default` |
| `Array.length arr` | Number of elements |
| `arr.(i)` | Read index `i` (`int`) |
| `arr.(i) <- v` | Write index `i` in place |

Element writes do **not** require `let mutable arr` — the slice cells are mutable. Use `mutable` only if you need to rebind the array variable itself.

See [`arrays.goop`](../examples/arrays.goop) and the full LUT example [`trading_decision_lut.goop`](../examples/trading_decision_lut.goop).

## Filling a LUT with `for`

```goop
let lut = Array.make 100 default in
for i = 0 to 99 do
  lut.(i) <- compute i
done
```

- Bounds `from` and `to` must be `int`.
- The loop is **inclusive** on both ends (`0 .. 99` runs 100 times).
- The loop variable (`i` here) is visible only inside `do ... done`.

## Sequencing with `begin` / `end`

When a `let` body needs several steps before a result:

```goop
let sum =
  begin
    arr.(0) <- 1;
    arr.(1) <- 2;
    arr.(0) + arr.(1)
  end
```

Statements separated by `;` run in order. The last expression is the value of the whole `begin ... end`.

## Qualified constructors

Disambiguate constructors in large modules:

```goop
type Color = Red | Green | Blue

let label c =
  match c with
  | Color.Red -> "red"
  | Color.Green -> "green"
  | Color.Blue -> "blue"
```

## Optional `std.array` import

Prelude bindings `Array.make` and `Array.length` need no import. For documentation-style qualified access:

```goop
import goop . "std.array"

let xs = make 3 0   (* same as Array.make *)
```

See [std.array](../stdlib/std-array.md).

## Common errors

| Message | Cause | Fix |
|---|---|---|
| `cannot assign to immutable binding` | `x <- ...` on non-`mutable` `let` | Use `let mutable x = ... in` |
| `type mismatch` on `arr.(i)` | Index or array type wrong | Ensure `i : int` and `arr : 'a array` |
| `expected DONE, got ...` | Missing `done` after `for` | Close the loop with `done` |

Full catalog: [error reference](../design/10-error-reference.md).

## Next steps

- [Safety checks](06-safety-checks.md) — exhaustiveness, races, nil channels
- [OCaml surface syntax](../design/13-ocaml-surface-syntax.md) — design rationale
- [Go lowering](../design/04-go-lowering.md) — how arrays and loops become Go
