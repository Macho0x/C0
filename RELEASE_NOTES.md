# Goop 1.5.0

Goop 1.5.0 hardens OCaml-style call lowering so libraries can drop `@[go]`
adapters that existed only to paper over codegen gaps.

## Highlights

- **Capitalized multi-arg apps:** `Add 2 3` → `Add(2, 3)`.
- **Unit erasure:** `unit` params are not Go parameters; `Now ()` → `time.Now()`,
  and `let f (_u: unit) = …` → `func F()`.
- **If-as-expression:** `let x = if c then a else b` emits a valid Go IIFE.
- **Option naming:** consistent `optionTypeSuffix` for generated helpers.

## Verification

- `go test ./...` (from `src/`)
- `goop test tests/` → 90 passed, 0 failed
- Treelog: `goop test .` → 4 passed, 0 failed (recompiled under 1.5)

## Treelog note

1.5 unblocks native Options/Scope/Handle work. Treelog still uses a hybrid
`implements` + `@[go]` Handle/sanitize layer; a follow-on pass migrates Options
(`'a option`), Scope (`Fail` + Lock/Unlock), and Handle to majority Goop using
these lowerings.
