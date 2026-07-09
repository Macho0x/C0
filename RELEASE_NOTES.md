# Goop 0.8.0

## OCaml surface syntax
- **Arrays**: `'a array`, `Array.make`, `Array.length`, `arr.(i)`, `arr.(i) <- v`
- **For loops**: `for i = lo to hi do ... done`
- **Sequencing**: `begin e1; e2; ... end`
- **Qualified constructors**: `Type.Constructor` in expressions and patterns

## Codegen fixes
- Top-level record literals emit direct values (not thunks)
- Parentheses preserve float operator precedence in generated Go
- `option` types in record fields register `OptionT` structs during prescan
- Tuple pattern matching on `(a, b)` with literal arms

## Tests & examples
- Rewritten [trading_decision_lut_test.goop](tests/trading_decision_lut_test.goop) using flat-array populate + overrides
- New integration tests: arrays, for loops, begin/end, qualified ctors, option records, paren floats, tuple match
- New example: [docs/examples/trading_decision_lut.goop](docs/examples/trading_decision_lut.goop)

## Documentation
- [docs/design/13-ocaml-surface-syntax.md](docs/design/13-ocaml-surface-syntax.md)
- [docs/stdlib/std-array.md](docs/stdlib/std-array.md)
- Updated grammar spec with new keywords and forms
