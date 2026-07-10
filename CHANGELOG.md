# Changelog

## 1.0.0

### Breaking — OCaml-aligned surface

Removed non-OCaml duplicate syntax (PARSE-MIG010–018):

- `let mutable` / binding `<-` → `ref` / `!` / `:=` (array `arr.(i) <-` kept)
- `?`, `result { }`, `async { }`, `region { }`, `guard` / `is` / expr `as` → `match`
- `newtype` → single-ctor ADT + `private`
- `with { io }` effect rows → OCaml 5-style `effect` / `perform` / handlers (CPS-lowered)
- `panic` / `panic_message` → `failwith`
- `%` → `mod` (`land` / `lor` / `lxor` added)

Kept: Go-style `import golang` / `import goop`, `go` / `go (move …)`, `@golang { }`, linear/`owned_chan`, `where` refinements.

### Added

- `while`, `function`, `exception` / `raise` / `try`/`with`/`finally`, `assert`, `lazy`, `//` comments
- Nested modules (`struct`/`sig`/`open`/`include`/functors/`let module`), `.mli` export check
- Polymorphic variants, GADTs (approx), labelled args, `[| … |]` array literals
- Minimal OCaml OOP (`class` / `object` / `new`)
- Effect handlers via CPS / free-monad lowering (effectful Go is not idiomatic by design)
- Optional Z3 SMT for refinements (`[check] smt = true`)
- Style guide: `docs/design/STYLE.md`, parity matrix: `docs/design/14-ocaml-parity.md`

### Examples & tests

- LUT decision logic refactored to `situation` ADT + `match`
- 49 e2e tests; new coverage for ref/while, exceptions, mod, function, array lit, effects, modules

## 0.8.2

### Documentation
- Sync all docs with compiler: version/chapter counts, `goop.toml` `deadlock` key, array/stdlib coverage
- Add NIL001, LINEAR008, DEADLOCK001 to error reference; fix trading-bot safety matrix (LINEAR001 typo)
- Replace stale `extern "go"` examples with `import golang` / `@golang` across syntax, lowering, and grammar docs
- Add arrays/for/begin/qualified constructors to `03-syntax.md`; array lowering to `spec/lowering.md`
- Update `prelude.md` and `builtins.md` with `Array.*`; rewrite Zed editor README for Goop

## 0.8.1

### Documentation & stdlib
- Tutorial chapter 7 (arrays and loops); semantics, lowering, and error-reference updates
- `std.array` thin re-export module; `goop.toml` mapping

### Tests
- Trading decision LUT test: qualified constructors, `begin`/`end` assertions
- Parser, typecheck, and fmt unit tests for OCaml surface syntax
- `std_array_test.goop`; `arrays.goop` example

## 0.8.0

### OCaml surface syntax
- Arrays via `Array.make`, `Array.length`, `arr.(i)`, `arr.(i) <- v`, and `'a array` types
- `for i = lo to hi do ... done` loops
- `begin ...; ... end` sequencing blocks
- Qualified constructors `Type.Ctor`

### Codegen
- Fix top-level record literal thunks, float paren precedence, option-in-record prescan, tuple match

### Tests & docs
- OCaml-style trading decision LUT test (375-cell universe)
- Eight new integration tests; design doc 13 and std-array reference
