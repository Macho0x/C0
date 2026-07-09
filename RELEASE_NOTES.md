# Goop 0.8.2

Documentation release — brings every doc surface in line with the compiler as of v0.8.1 features and v0.7.0 safety passes.

## Documentation sync

- **README** — v0.8.2 status, 7 tutorial chapters, 40+ e2e tests, `deadlock` in `goop.toml` example, arrays in feature table
- **Error reference** — NIL001 (nil channel), LINEAR008 (channel-mediated race), DEADLOCK001 (narrow static deadlock lint)
- **Trading bot safety** — fixed LINEAR001 typo, added LINEAR008, updated version label
- **Syntax reference** (`03-syntax.md`) — arrays, `for`/`done`, `begin`/`end`, qualified constructors; `import golang` / `@golang` replaces removed `extern "go"`
- **Lowering** (`04-go-lowering.md`, `spec/lowering.md`) — inline Go via `@golang`; array/for/begin lowering in spec
- **Stdlib** — `Array.make` / `Array.length` in prelude reference; `'a array` in builtins
- **Grammar** (`spec/grammar.md`) — reserved words, `'a array` type, removed legacy `extern_decl`
- **Semantics** — dot import replaces legacy `open`
- **Design analysis** (`09-compile-time-checks-analysis.md`) — LINEAR008 and DEADLOCK001 marked implemented (v0.7.0)
- **Tutorial** ch. 5–6 — LINEAR008 and DEADLOCK001 coverage; `deadlock` config key
- **Zed README** — corrected for Goop compiler and extension paths

## No compiler changes

This release is documentation-only. Binary artifacts are rebuilt from the same source tree.

## 0.8.1 (previous release)

Follow-up to 0.8.0 — documentation, `std.array`, and test polish for the OCaml surface syntax.

### Documentation
- [Tutorial ch. 7](docs/tutorial/07-arrays-and-loops.md): arrays, `for`, `begin`/`end`
- [Semantics](docs/spec/semantics.md) and [Go lowering](docs/design/04-go-lowering.md) for arrays/loops
- [Error reference](docs/design/10-error-reference.md): assignment and index parse errors (TYPE011–013, PARSE023)

### std.array
- Thin wrapper at `std/array/array.goop` re-exporting prelude `Array.make` / `Array.length` as `make` / `length`
- Import: `import goop . "std.array"`

### Tests
- [trading_decision_lut_test.goop](tests/trading_decision_lut_test.goop): qualified constructors, `begin`/`end` test body
- Unit tests in parser, typecheck, and fmt for index/assign, for, begin/end, qualified ctors
- New example: [docs/examples/arrays.goop](docs/examples/arrays.goop)
