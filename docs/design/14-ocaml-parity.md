# OCaml Parity Matrix (Goop 1.0)

Status legend: `✅` same · `🔄` different-by-design · `➕` Goop extension · `🆕` added in 1.0 · `❌` out of scope

## Expressions & control

| OCaml | Goop | Status |
|-------|------|--------|
| `let` / `let rec` / `and` / `in` | Same | ✅ |
| `fun` / `function` | Same | 🆕 `function` |
| `if then else` | Same | ✅ |
| `match` / `when` / `as` (pattern) | Same | ✅ |
| `begin … end` | Same | ✅ |
| `for … to … do … done` | Same | ✅ |
| `while … do … done` | Same | 🆕 |
| `ref` / `!` / `:=` | Same | 🆕 |
| `arr.(i)` / `arr.(i) <-` | Same | ✅ |
| `[| … |]` const arrays | Supported | 🆕 |
| `lazy` | Supported | 🆕 |
| `try … with` / `raise` / `exception` | Same | 🆕 |
| `failwith` | Same | 🆕 |
| `\| A \| B ->` or-patterns | Same | 🆕 |
| `~label` / `?optional` | Same | 🆕 |
| `\|>` | Same | ✅ |
| `;;` | Not used (file = module) | 🔄 |
| `//` comments | Supported (+ `(* *)`) | 🆕 |

## Types

| OCaml | Goop | Status |
|-------|------|--------|
| ADTs / records / tuples | Same | ✅ |
| `'a option` / `result` | Builtins | ✅ |
| Polymorphic variants | Supported | 🆕 |
| GADTs | Supported | 🆕 |
| Object / class types | Supported | 🆕 |
| `private` types | Supported | 🆕 |
| `where` refinements + SMT | Extended | 🆕 |
| Effect handlers (OCaml 5) | CPS-lowered | 🆕 |

## Modules

| OCaml | Goop | Status |
|-------|------|--------|
| `module M = struct … end` | Supported | 🆕 |
| `module type` / `sig` | Supported | 🆕 |
| `open` / `include` | Supported | 🆕 |
| Functors | Supported | 🆕 |
| `let module` | Supported | 🆕 |
| `.mli` | Supported | 🆕 |
| `import golang` / `import goop` | Goop-only | ➕ |
| `go` / `chan` / `move` | Goop-only | ➕ |

## Intentionally different

| Topic | Notes |
|-------|-------|
| Effectful codegen | CPS / free-monad Go (not idiomatic); pure code stays direct-style |
| Imports | Go-style paths kept |
| Concurrency | `go` / channels, not OCaml Domains |
| Stdlib | Thin prelude + `import golang`; not full OCaml stdlib |

## Removed (non-OCaml duplicates)

`let mutable`, `?`, `result{}`/`async{}`/`region{}`, `is`/`as`/`guard` macros, `newtype`, `with { io }` rows, `panic` keyword, `%` (use `mod`).
