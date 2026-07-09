# OCaml Surface Syntax (v0.8.0)

Goop standardizes on **one OCaml-canonical imperative surface** for LUTs, loops, and mutation. Rust/Go-style alternatives (`arr[i]`, C `for(;;)`, brace blocks) are intentionally rejected.

## Arrays

- Type: `'a array` (postfix, like OCaml `int array`)
- Create: `Array.make n default` → Go `make([]T, n)` filled with `default`
- Length: `Array.length arr` → Go `len(arr)`
- Read: `arr.(i)` → Go `arr[i]`
- Write: `arr.(i) <- v` → Go `arr[i] = v`

Const-size arrays (`[| ... |]`, `[T; N]`) remain **deferred** — see [08-deferred-features-analysis.md](08-deferred-features-analysis.md).

## For loops

```goop
for i = 0 to n - 1 do
  arr.(i) <- f i
done
```

Lowers to Go `for i := 0; i <= n-1; i++ { ... }`.

## Sequencing

```goop
begin
  stmt1;
  stmt2;
  result_expr
end
```

Lowers to a block; the last expression is the value (or `return` inside functions).

## Qualified constructors

```goop
Color.Red
match c with Color.Green -> ...
```

## Codegen fixes (0.8.0)

- Top-level record literals no longer become thunks
- `(a -. b) /. c` preserves float precedence via Go parentheses
- `option` in record fields registers `OptionT` structs during prescan
- Tuple `match` on `(a, b)` with literal patterns

## LUT pattern

See [trading_decision_lut.goop](../examples/trading_decision_lut.goop) and [trading_decision_lut_test.goop](../../tests/trading_decision_lut_test.goop).
