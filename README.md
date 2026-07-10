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
  <strong>Catch production bugs before <code>go build</code>.</strong><br>
  OCaml-aligned types and pattern matching on a Go runtime — with compile-time<br>
  exhaustiveness, refinements (optional Z3), branded ADTs, and data-race detection.
</p>

<p align="center">
  <a href="#how-goop-compares">Compare</a> •
  <a href="#trading-bot-safety">Trading bots</a> •
  <a href="#what-goop-catches">Compile-time safety</a> •
  <a href="#go-interop">Go interop</a> •
  <a href="#inline-go">Inline Go</a> •
  <a href="#getting-started">Getting Started</a> •
  <a href="#documentation">Docs</a>
</p>

---

## How Goop compares

Goop targets **compile-time** checks that Go leaves to runtime tests, panics, or `-race`. Refinements erase to guards (or nothing when proven); see [lowering](docs/design/04-go-lowering.md). Style: [STYLE.md](docs/design/STYLE.md).

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

Goop is not trying to replace Rust’s ownership model. The pitch is **ML-family safety on a Go runtime** — catch whole classes of production bugs before `go build`, without leaving the Go ecosystem.

Design influences: [language overview](docs/design/01-overview.md) · [compile-time checks analysis](docs/design/09-compile-time-checks-analysis.md) · [OCaml parity](docs/design/14-ocaml-parity.md)

---

## Trading bot safety

Condensed from the [full trading-bot matrix](docs/design/12-trading-bot-safety.md). These are failure modes that show up constantly in venue integrations and order routers.

| Failure mode | Go at compile time | Goop at compile time |
|---|---|---|
| Unhandled order ack / venue message variant | ❌ | ✅ EXHAUST003 |
| `order_id` swapped with `symbol` (both `string`) | ❌ | ✅ branded ADTs |
| Send/recv on nil channel | ❌ | ✅ NIL001 |
| Shared `ref` state captured by `go` | ❌ (`-race` at runtime) | ✅ linear pass |
| Pure helper secretly doing IO | ❌ | ⚠️ discipline / handlers |
| Division when divisor may be zero | ❌ | ⚠️ REFINE001 / REFINE002 |

Examples: [`trading_order_ack_test.goop`](tests/trading_order_ack_test.goop) · [`newtype_trading.goop`](docs/examples/newtype_trading.goop) · [`trading_binance.goop`](tests/trading_binance.goop)

---

## What Goop catches

Goop is not “Go with nicer syntax.” The compiler runs a unified safety pipeline on every `goop check`, `goop build`, and LSP diagnostic — before your code reaches `go build`.

| Check | Code | What it prevents |
|---|---|---|
| Exhaustive `match` | EXHAUST003 | Unhandled ADT variants in trading bots and HTTP handlers |
| Branded ADTs | type error | Swapping `order_id` and `symbol` (both `string` in Go) |
| Nil channels | NIL001 | Sends/receives on channels before initialization |
| Goroutine sharing | linear pass (LINEAR006/007) | `ref` captured by `go`; use `go (move x)` to transfer |
| Refinement contracts | refine pass (REFINE001–002) | Proven VCs skip guards; unproven emit call-site checks |

Full matrix: [Trading bot safety](docs/design/12-trading-bot-safety.md) · [Error reference](docs/design/10-error-reference.md)

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
FAIL: exhaustiveness errors:
✕ EXHAUST003: non-exhaustive match: missing constructor(s): PartialFill
╭─[handler.goop:8:3]
  7 │ let handleAck (ack: OrderAck) : string =
> 8 │   match ack with
  ·   ╰── EXHAUST003: non-exhaustive match: missing constructor(s): PartialFill
  9 │   | Filled { order_id; qty } -> order_id
╰────
```

See [`tests/trading_order_ack_test.goop`](tests/trading_order_ack_test.goop) for the correct version.

### Nominal branding

Brand trading IDs so raw strings cannot slip through:

```goop
type order_id = Order_id of string
type symbol = Symbol of string

let place (sym: symbol) (oid: order_id) : string =
  "order on branded ids"
```

Assigning a bare string to `order_id` is rejected. Example: [`docs/examples/newtype_trading.goop`](docs/examples/newtype_trading.goop)

### Data races at compile time

```goop
let race () : unit =
  let counter = ref 0 in
  let ignored = go (fun () -> print_line (int_to_string (!counter))) in
  print_line (int_to_string (!counter))   (* still in scope — rejected *)
```

```
FAIL: linear discharge errors:
potential data race: mutable variable "counter" captured by goroutine
is still accessible in spawning scope
```

Go’s race detector only fires at runtime. Goop flags the pattern at compile time. Use `go (move counter)` when the parent no longer needs the binding. Examples: [`race_detection.goop`](docs/examples/race_detection.goop) · [`go_move.goop`](docs/examples/go_move.goop)

---

## Go interop

Use any Go package from Goop. Mixed `.goop` + `.go` projects work in the same module — migrate file by file.

```goop
module main

import (
  golang "strings" {
    val ToUpper : string -> string
    val ToLower : string -> string
  }
  golang "fmt"
  goop . "std.io"
)

let main () : unit =
  print_line (ToUpper "hello from Goop")
```

- **`import golang "path"`** — import-only; use with `@golang` blocks or generated bindings.
- **`import golang "path" { val F : ... }`** — declare signatures for Go functions you call from Goop.
- **`import goop "my/pkg"`** — other Goop modules in the same project.

Full example: [`docs/examples/extern_demo.goop`](docs/examples/extern_demo.goop)

---

## Inline Go

Embed multi-statement Go without changing the compiler. Declare bindings with `@golang { ... }` and expose them to Goop with `val` signatures:

```goop
@golang {
  func greet(name string) string {
    return "Hello, " + name + "!"
  }
}
val greet : string -> string

let main () : unit =
  print_line (greet "Goop")
```

Same file can mix `import golang`, `@golang` blocks, and pure Goop logic. See [`docs/examples/extern_demo.goop`](docs/examples/extern_demo.goop).

---

## Language features (condensed)

| Area | Highlights |
|---|---|
| **Types** | ADTs, records, tuples, lists, `option` / `result`, branded single-ctor ADTs, parametric polymorphism |
| **Control** | Exhaustive `match`, `function`, `when`, `try`/`with`, `failwith`, `while`/`for`/`begin` |
| **Mutation** | `ref` / `!` / `:=`, `mutable` fields, `arr.(i) <-` |
| **Safety** | Linear resources (`owned_chan`), refinements (`where` + optional Z3), nil-channel analysis |
| **Effects** | OCaml 5-style `effect` / `perform` / handlers (CPS-lowered) |
| **Arrays & loops** | `'a array`, `Array.make`, `for`/`while`/`done`, qualified constructors |
| **Concurrency** | `go`, `chan`, `select`, `go (move …)` |
| **Interop** | `import golang`, `import goop`, `@golang` embed blocks |

Style guide: [docs/design/STYLE.md](docs/design/STYLE.md). More examples: [`docs/examples/`](docs/examples/).

---

## What the generated Go looks like

Goop lowers to idiomatic Go — interface sum types, type switches, no runtime library:

```goop
type option = | Some of { value: int } | None

let map_option (opt: option) (f: int -> int) : option =
  match opt with
  | Some { value } -> Some { value = f value }
  | None -> None
```

```go
func mapOption(opt Option, f func(int) int) Option {
    switch o := opt.(type) {
    case OptionSome:
        return OptionSome{Value: f(o.Value)}
    case OptionNone:
        return OptionNone{}
    default:
        panic("unreachable: exhaustive match")
    }
}
```

Details: [Go lowering strategy](docs/design/04-go-lowering.md)

---

## Getting started

```bash
# Build the compiler
cd src && go build -o ../goop ./cmd/goop

# Type-check a file
../goop check hello.goop

# Run tests
../goop test ../tests/

# LSP (for editors)
../goop lsp
```

Configure checks in `goop.toml`:

```toml
[check]
exhaust_redundant = "warn"
exhaust_missing = "error"
concurrent = "error"          # LINEAR006/007/008: warn | error | off
refinement_unproven = "warn"  # REFINE002: warn | error | off
deadlock = "warn"             # DEADLOCK001: warn | error | off
smt = false                   # optional Z3 for refinements
```

See [package manager guide](docs/design/11-package-manager.md).

---

## FAQ

**Is this production-ready?** Early bootstrap toward **1.0**. The compiler, type checker, codegen, LSP, and e2e tests are real; we are not claiming production load readiness yet.

**How is this different from Borgo or Dingo?** Goop is a full compiler (lex → parse → infer → safety passes → Go codegen), with OCaml-aligned syntax and compile-time safety aimed at gradual migration to Go. Dingo-style `?` and F# computation expressions were removed in 1.0.

**Do I need OCaml?** No. If you know pattern matching from Rust, Swift, or Kotlin, the syntax will feel familiar. See [STYLE.md](docs/design/STYLE.md).

**Do I need a new stdlib?** No. Call Go via `import golang` or use shipped `std.*` modules (`std.io`, `std.list`, …).

**Do I need Z3?** No. Install Z3 only if you enable `[check] smt = true`.

---

## Status

**Toward 1.0** — OCaml-aligned surface (`ref`, `while`, exceptions, effect handlers, SMT refinements); removed F#/Kit/Dingo sugar. See [STYLE.md](docs/design/STYLE.md) and [14-ocaml-parity.md](docs/design/14-ocaml-parity.md).

**v1.0.0** — OCaml-aligned surface (`ref`/`while`/`function`/exceptions/modules), CPS effect handlers, optional Z3 SMT; removed non-OCaml sugar. See [STYLE.md](docs/design/STYLE.md).

**v0.8.x** — Arrays/`for`/`begin`, tutorial, error reference, trading LUT examples.

- 🔮 Self-hosting compiler in Goop

---

## Documentation

| Resource | Link |
|---|---|
| [Language tutorial](docs/tutorial/README.md) | 7 chapters, linked examples |
| [Standard library reference](docs/stdlib/README.md) | Prelude, builtins, `std.*` |
| [Design docs](docs/design/) | Language design ([STYLE](docs/design/STYLE.md), [parity](docs/design/14-ocaml-parity.md)) |
| [Examples](docs/examples/) | Runnable; CI checks all |
| [Grammar](docs/spec/grammar.md) | Draft |
| [Contributing](CONTRIBUTING.md) | Build, editors, doc workflow |
| [GitHub highlighting](docs/github-linguist/README.md) | Linguist submission (F# interim via `.gitattributes`) |
| Documentation generator (`goop doc`) | Not started |

---

## License

MIT / Apache-2.0 dual license.
