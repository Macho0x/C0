# Goop 1.0.0

OCaml-aligned mega release: one canonical surface per task (OCaml spelling), with Go imports and `go`/`move` kept as intentional extensions.

## Highlights

- **Breaking cleanup** of F#/Kit/Dingo sugar (`?`, CEs, `is`/`guard`, `newtype`, `let mutable`, effect rows, `panic`, `%`)
- **OCaml additions:** `ref`/`!`/`:=`, `while`, `function`, exceptions, modules/functors, poly variants/GADTs (minimal), OOP (minimal), `[| |]`, `lazy`, `mod`/bitwise ops, `//` comments
- **Effects:** OCaml 5-style handlers lowered via **CPS** (effectful Go is not idiomatic; pure code stays direct-style)
- **SMT:** optional Z3 for `where` refinements (`[check] smt = true`)
- **Docs:** [STYLE.md](docs/design/STYLE.md), [14-ocaml-parity.md](docs/design/14-ocaml-parity.md), updated tutorial/spec/design

## Migration

| Old | New |
|-----|-----|
| `let mutable x = 0` / `x <- 1` | `let x = ref 0` / `x := 1` / `!x` |
| `e ?` / `result { }` | `match e with \| Ok x -> … \| Error e -> …` |
| `type t = newtype string` | `type t = T of string` |
| `f : t with { io }` | drop row; use `effect` / handlers |
| `panic "…"` | `failwith "…"` |
| `a % b` | `a mod b` |

## Verification

- `go test ./...` (from `src/`)
- `goop test tests/` — 49 passed
- `goop check` on all `docs/examples/*.goop`
