# 19. Call lowering: capitalized apps, unit, if-expressions

**Status:** Shipped in Goop **1.5.0**.

## Motivation

OCaml-style juxtaposition and `unit` must lower to idiomatic Go. Treelog and
similar libraries use capitalized multi-arg lets (`Emit_request a b c d`),
`Now ()`, and `let x = if …`. Bugs here forced large `@[go]` adapters.

## Rules (OCaml-faithful)

| Goop | Go |
|------|-----|
| `Add 2 3` (capitalized let) | `Add(2, 3)` — not `Add(2)(3)` |
| `Now ()` / `f ()` when params are `unit` | `time.Now()` / `F()` — no `struct{}{}` |
| `let f (_u : unit) = …` | `func F()` — unit params erased |
| `let x = if c then a else b` | IIFE `func() T { if c { return a }; return b }()` |
| `colors.gray ()` (cross-package) | `colors.Gray()` — unit args stripped |

Capitalized names are parsed as `ConstructorExpr` with an embedded first
argument. Codegen flattens that embedding for user lets and Go externs, but
**not** for ADT / `None` / `Some` / `Ok` / `Error` constructors.

## Option type naming

`internalTypeToGo` for `'a option` must use the same `optionTypeSuffix` as
helper emission (`OptionQuoteparams`, not `OptionQuote_params`).

## Non-goals

- `defer`
- Changing OCaml object `#method` syntax
- Auto-import of all Go package exports

See also: [04-go-lowering.md](04-go-lowering.md), [18-go-methods.md](18-go-methods.md).
