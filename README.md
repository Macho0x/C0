<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/goop-banner.png">
    <source media="(prefers-color-scheme: light)" srcset="assets/goop-banner-whitebg.jpg">
    <img alt="Goop banner" src="assets/goop-banner-whitebg.jpg" width="680">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/Macho0x/Goop/actions/workflows/ci.yml">
    <img alt="CI status" src="https://github.com/Macho0x/Goop/workflows/CI/badge.svg">
  </a>
  <a href="https://github.com/Macho0x/Goop/releases">
    <img alt="Latest release" src="https://img.shields.io/github/v/release/Macho0x/Goop?display_name=tag&label=release">
  </a>
  <img alt="License" src="https://img.shields.io/badge/license-MIT%2FApache--2.0-blue">
</p>

<h1 align="center">Goop</h1>

<p align="center">
  <strong>OCaml-style safety on Go’s runtime.</strong>
</p>

- Exhaustive ADTs and pattern matching
- Branded IDs (single-ctor ADTs) so `order_id` ≠ `symbol`
- Go interop and deploy — same runtime, stdlib, and binaries

### Non-exhaustive match (EXHAUST003)

```goop
type OrderAck =
  | Filled of { order_id: string; qty: int }
  | Rejected of { reason: string }
  | PartialFill of { order_id: string; filled: int; remaining: int }

let handleAck (ack: OrderAck) : string =
  match ack with
  | Filled { order_id; qty } -> order_id
  | Rejected { reason } -> reason
  (* forgot PartialFill — Go compiles; Goop does not *)
```

```
✕ EXHAUST003: non-exhaustive match: missing constructor(s): PartialFill
```

## Getting started

```bash
cd src && go build -o ../goop ./cmd/goop
../goop check hello.goop
```

Full walkthrough: [Language tutorial](docs/tutorial/README.md)

<p align="center">
  <a href="docs/tutorial/README.md">Tutorial</a> ·
  <a href="docs/design/STYLE.md">STYLE</a> ·
  <a href="docs/examples/">Examples</a> ·
  <a href="https://github.com/Macho0x/Goop/releases">Releases</a>
</p>

---

## How Goop compares

Goop targets **compile-time** checks that Go leaves to runtime tests, panics, or `-race`. Style: [STYLE.md](docs/design/STYLE.md).

| Safety feature | Go | Rust | OCaml | Goop |
|---|---|---|---|---|
| Sum types + exhaustive `match` | ❌ | ✅ | ✅ | ✅ |
| No null by default (`option`) | ❌ | ✅ | ✅ | ✅ |
| Branded IDs (single-ctor ADT) | ❌ | ✅ | ✅ | ✅ |
| Effect handlers (OCaml 5-style) | ❌ | ❌ | ✅ | ✅ (CPS-lowered) |
| Nil channel misuse | ❌ (blocks/panics at runtime) | N/A | N/A | ✅ (NIL001) |
| Data race detection | ⚠️ runtime (`-race`) | ✅ (borrow checker) | ❌ | ✅ (linear pass, conservative) |
| Refinement / contract checks | ❌ | ⚠️ limited | ⚠️ limited | ✅ VC + optional Z3 + guards |
| Native Go stdlib + deploy model | ✅ | ❌ | ❌ | ✅ |

✅ compile-time · ⚠️ partial or runtime · ❌ not available

## Trading bot safety

Venue integrations and order routers fail on unhandled ADT variants, swapped string IDs, and nil channels — Goop catches those at compile time. Full matrix: [12-trading-bot-safety.md](docs/design/12-trading-bot-safety.md). Examples: [`branded_ids.goop`](docs/examples/branded_ids.goop) · [`trading_order_ack_test.goop`](tests/trading_order_ack_test.goop).

## Go interop

Call any Go package and embed Go when needed — migrate file by file in the same module.

```goop
module main

import (
  golang "strings" {
    val ToUpper : string -> string
  }
)

@golang {
  func greet(name string) string {
    return "Hello, " + name + "!"
  }
}
val greet : string -> string

let main () : unit =
  print_line (greet (ToUpper "goop"))
```

- **`import golang "path"`** — Go packages (optional `{ val … }` signatures)
- **`import goop "path"`** — other Goop modules
- **`@golang { … }`** — embed multi-statement Go in the same file

See [`extern_demo.goop`](docs/examples/extern_demo.goop).

## Language features

Everyday surface: [STYLE.md](docs/design/STYLE.md) · tutorial · [`docs/examples/`](docs/examples/) (`branded_ids`, `match_patterns`, `linear_resource`, `chan_async`, `result_match`, `effects`).

Highlights: ADTs + exhaustive `match`, `ref`/`!`/`:=`, `go`/`chan`/`go (move …)`, linear resources, `where` refinements (optional Z3), minimal `effect`/`perform` handlers (CPS-lowered).

## FAQ

**Is 1.1.0 production-ready?** Current release is shipped (compiler, type checker, codegen, LSP, e2e tests). Some OCaml features are pragmatic subsets (GADTs, objects, shallow effects); we are not claiming production load readiness yet.

**How is this different from Borgo or Dingo?** Goop is a full compiler with OCaml-aligned syntax and compile-time safety for gradual migration to Go. Dingo-style `?` and F# computation expressions were removed in 1.0.

**Do I need OCaml or Z3?** No. Pattern matching from Rust/Swift/Kotlin transfers. Z3 is optional (`[check] smt = true`).

## Status

**v1.1.0** — OCaml parity mega-ship (modules, types, objects, shallow effects). See [CHANGELOG](CHANGELOG.md) · [parity](docs/design/14-ocaml-parity.md).

**v1.0.1** — Docs/README cleanup, example renames, real effects demo.

**v1.0.0** — OCaml-aligned surface, CPS effect handlers, optional Z3 SMT.

## Documentation

| Resource | Link |
|---|---|
| [Language tutorial](docs/tutorial/README.md) | 7 chapters, linked examples |
| [Standard library reference](docs/stdlib/README.md) | Prelude, builtins, `std.*` |
| [Design docs](docs/design/) | [STYLE](docs/design/STYLE.md), [parity](docs/design/14-ocaml-parity.md) |
| [Examples](docs/examples/) | Runnable; CI checks all |
| [Grammar](docs/spec/grammar.md) | Draft |
| [Contributing](CONTRIBUTING.md) | Build, editors, doc workflow |

## License

MIT / Apache-2.0 dual license.
