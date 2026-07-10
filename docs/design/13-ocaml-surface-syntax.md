# OCaml Surface Syntax (1.0)

Goop standardizes on **one OCaml-canonical surface**. Rust/Go-style alternatives (`arr[i]`, C `for(;;)`, brace blocks as primary) are rejected.

v0.8 introduced arrays, `for`/`done`, `begin`/`end`, and qualified constructors. **Goop 1.0 expands** that surface to full OCaml alignment for everyday code: `ref`/`!`/`:=`, `while`, `function`, exceptions, `failwith`, `mod`, modules/`sig`/functors, objects, effect handlers, SMT refinements, and removal of F#/Kit/Dingo duplicates. See [STYLE.md](STYLE.md) and [14-ocaml-parity.md](14-ocaml-parity.md).

## Arrays

- Type: `'a array` (postfix, like OCaml `int array`)
- Create: `Array.make n default` → Go `make([]T, n)` filled with `default`
- Length: `Array.length arr` → Go `len(arr)`
- Read: `arr.(i)` → Go `arr[i]`
- Write: `arr.(i) <- v` → Go `arr[i] = v`
- Const arrays: `[| … |]` supported in 1.0

## Refs and while

```goop
let r = ref 0 in
while !r < n do
  r := !r + 1
done
```

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

## Qualified constructors

```goop
Color.Red
match c with Color.Green -> ...
```

Prefer **PascalCase type names** for qualified forms. Lowercase-type qualified constructors may codegen poorly — keep those unqualified until mapped to `New…` helpers.

## LUT pattern

See [trading_decision_lut.goop](../examples/trading_decision_lut.goop) and [trading_decision_lut_test.goop](../../tests/trading_decision_lut_test.goop).
