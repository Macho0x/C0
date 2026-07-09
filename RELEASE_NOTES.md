# Goop 0.8.1

Follow-up to 0.8.0 — documentation, `std.array`, and test polish for the OCaml surface syntax.

## Documentation
- [Tutorial ch. 7](docs/tutorial/07-arrays-and-loops.md): arrays, `for`, `begin`/`end`
- [Semantics](docs/spec/semantics.md) and [Go lowering](docs/design/04-go-lowering.md) for arrays/loops
- [Error reference](docs/design/10-error-reference.md): assignment and index parse errors (TYPE011–013, PARSE023)

## std.array
- Thin wrapper at `std/array/array.goop` re-exporting prelude `Array.make` / `Array.length` as `make` / `length`
- Import: `import goop . "std.array"`

## Tests
- [trading_decision_lut_test.goop](tests/trading_decision_lut_test.goop): qualified constructors (`PriceVsMean.FarAbove`, `TradeSide.Buy`, …), `begin`/`end` test body
- Unit tests in parser, typecheck, and fmt for index/assign, for, begin/end, qualified ctors
- New example: [docs/examples/arrays.goop](docs/examples/arrays.goop)

## 0.8.0 (previous release)

### OCaml surface syntax
- **Arrays**: `'a array`, `Array.make`, `Array.length`, `arr.(i)`, `arr.(i) <- v`
- **For loops**: `for i = lo to hi do ... done`
- **Sequencing**: `begin e1; e2; ... end`
- **Qualified constructors**: `Type.Constructor` in expressions and patterns

### Codegen fixes
- Top-level record literals emit direct values (not thunks)
- Parentheses preserve float operator precedence in generated Go
- `option` types in record fields register `OptionT` structs during prescan
- Tuple pattern matching on `(a, b)` with literal arms
