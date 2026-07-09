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
  OCaml-style types and pattern matching on a Go runtime — with compile-time<br>
  exhaustiveness, effect tracking, newtypes, and data-race detection.
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

Goop targets **compile-time** checks that Go leaves to runtime tests, panics, or `-race`. Effect rows and refinements are verified in Goop source; they erase in generated Go (see [lowering](docs/design/04-go-lowering.md)).

| Safety feature | Go | Rust | OCaml / F# | Goop |
|---|---|---|---|---|
| Sum types + exhaustive `match` | ❌ | ✅ | ✅ | ✅ |
| No null by default (`option`) | ❌ | ✅ | ✅ | ✅ |
| Branded / newtype IDs | ❌ | ✅ | ✅ | ✅ |
| Effect tracking in types | ❌ | ❌ | ✅ (F#) | ✅ (`with { io }`) |
| Nil channel misuse | ❌ (blocks/panics at runtime) | N/A | N/A | ✅ (NIL001) |
| Data race detection | ⚠️ runtime (`-race`) | ✅ (borrow checker) | ❌ | ✅ (linear pass, conservative) |
| Refinement / contract checks | ❌ | ⚠️ limited | ⚠️ limited | ✅ compile-time VC + call-site guards |
| Native Go stdlib + deploy model | ✅ | ❌ | ❌ | ✅ |

✅ compile-time · ⚠️ partial or runtime · ❌ not available

Goop is not trying to replace Rust’s ownership model. The pitch is **ML-family safety on a Go runtime** — catch whole classes of production bugs before `go build`, without leaving the Go ecosystem.

Design influences: [language overview](docs/design/01-overview.md) · [compile-time checks analysis](docs/design/09-compile-time-checks-analysis.md)

---

## Trading bot safety

Condensed from the [full trading-bot matrix](docs/design/12-trading-bot-safety.md). These are failure modes that show up constantly in venue integrations and order routers.

| Failure mode | Go at compile time | Goop at compile time |
|---|---|---|
| Unhandled order ack / venue message variant | ❌ | ✅ EXHAUST003 |
| `order_id` swapped with `symbol` (both `string`) | ❌ | ✅ newtypes |
| Send/recv on nil channel | ❌ | ✅ NIL001 |
| Shared `mutable` state captured by `go` | ❌ (`-race` at runtime) | ✅ linear pass |
| Pure helper secretly doing IO | ❌ | ✅ UNIFY018 |
| Division when divisor may be zero | ❌ | ⚠️ REFINE001 / REFINE002 |

Examples: [`trading_order_ack_test.goop`](tests/trading_order_ack_test.goop) · [`newtype_trading.goop`](docs/examples/newtype_trading.goop) · [`trading_binance.goop`](tests/trading_binance.goop)

---

## What Goop catches

Goop is not “Go with nicer syntax.” The compiler runs a unified safety pipeline on every `goop check`, `goop build`, and LSP diagnostic — before your code reaches `go build`.

| Check | Code | What it prevents |
|---|---|---|
| Exhaustive `match` | EXHAUST003 | Unhandled ADT variants in trading bots and HTTP handlers |
| Effect rows | UNIFY018 / UNIFY019 | Pure functions secretly doing IO or spawning goroutines |
| Nominal newtypes | type error | Swapping `order_id` and `symbol` (both `string` in Go) |
| Nil channels | NIL001 | Sends/receives on channels before initialization |
| Goroutine sharing | linear pass (LINEAR006/007) | `mutable` captured by `go`; use `go (move x)` to transfer |
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

### Effect inference (UNIFY018)

Functions default to pure unless you declare effects. The compiler infers what the body actually does:

```goop
let helper () : unit with { } =
  print_line "needs io"   (* compile error *)

let main () : unit with { io } =
  helper ()
```

```
✕ UNIFY018: function declared pure `with {}` but body uses effects: io
```

Annotate IO helpers with `with { io }`, async code with `with { async }`, or both: `with { async; io }`.

### Nominal newtypes

Brand trading IDs so raw strings cannot slip through:

```goop
type order_id = newtype string
type symbol = newtype string

let place (sym: symbol) (oid: order_id) : string =
  "order on branded ids"
```

Assigning a bare string to `order_id` is rejected:

```
type mismatch: got order_id, expected string (expected primitive type)
```

Example: [`docs/examples/newtype_trading.goop`](docs/examples/newtype_trading.goop)

### Data races at compile time

```goop
let race () : unit with { async; io } =
  let mutable counter = 0 in
  let ignored = go (fun () -> print_line (int_to_string counter)) in
  print_line (int_to_string counter)   (* still in scope — rejected *)
```

```
FAIL: linear discharge errors:
potential data race: mutable variable "counter" captured by goroutine
is still accessible in spawning scope
```

Go’s race detector only fires at runtime. Goop flags the pattern at compile time. Use `go (move counter)` when the parent no longer needs a mutable binding. Examples: [`race_detection.goop`](docs/examples/race_detection.goop) · [`go_move.goop`](docs/examples/go_move.goop)

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

let main () : unit with { io } =
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

let main () : unit with { io } =
  print_line (greet "Goop")
```

Same file can mix `import golang`, `@golang` blocks, and pure Goop logic. See [`docs/examples/extern_demo.goop`](docs/examples/extern_demo.goop).

---

## Language features (condensed)

| Area | Highlights |
|---|---|
| **Types** | ADTs, records, tuples, lists, `option` / `result`, parametric polymorphism |
| **Control** | Exhaustive `match`, guards, active patterns, `?` propagation, `result { }` / `async { }` |
| **Safety** | Effect rows, linear resources (`owned_chan`), refinements (`where`), nil-channel analysis |
| **Concurrency** | `go`, `chan`, `select`, compile-time sharing analysis on `mutable` |
| **Interop** | `import golang`, `import goop`, `@golang` embed blocks |

More examples: [`docs/examples/`](docs/examples/) — ADTs, channels, refinements, trading bots, package manager demo.

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
effect_inference = true
concurrent = "error"          # LINEAR006/007: warn | error | off
refinement_unproven = "warn"  # REFINE002: warn | error | off
```

See [package manager guide](docs/design/11-package-manager.md).

---

## FAQ

**Is this production-ready?** Early bootstrap. The compiler, type checker, codegen, LSP, and 28+ e2e tests are real; we are not claiming production load readiness yet.

**How is this different from Borgo or Dingo?** Goop is a full compiler (lex → parse → infer → safety passes → Go codegen), with ML-family syntax and compile-time safety aimed at gradual migration to Go.

**Do I need OCaml?** No. If you know pattern matching from Rust, Swift, or Kotlin, the syntax will feel familiar.

**Do I need a new stdlib?** No. Call Go via `import golang` or use shipped `std.*` modules (`std.io`, `std.list`, …).

---

## Status

**v0.7.0** — `src/internal/fmt` package, channel-mediated race tracking (LINEAR008), narrow deadlock lint (DEADLOCK001).

- Language comparison table (Go / Rust / OCaml / F# / Goop) on README
- Trading bot safety summary (Go vs Goop compile-time checks)
- Banner: `<picture>` light/dark variants; renamed `goop-banner-whitebg.jpg`
- Prior: Goopher branding, Zed icons, Linguist color `#62c52e` (v0.5.3)
- 🔮 Self-hosting compiler in Goop

---

## Documentation

| Resource | Link |
|---|---|
| [Language tutorial](docs/tutorial/README.md) | 6 chapters, linked examples |
| [Standard library reference](docs/stdlib/README.md) | Prelude, builtins, `std.*` |
| [Design docs](docs/design/) | Language design |
| [Examples](docs/examples/) | Runnable; CI checks all |
| [Grammar](docs/spec/grammar.md) | Draft |
| [Contributing](CONTRIBUTING.md) | Build, editors, doc workflow |
| [GitHub highlighting](docs/github-linguist/README.md) | Linguist submission (F# interim via `.gitattributes`) |
| Documentation generator (`goop doc`) | Not started |

---

## License

MIT / Apache-2.0 dual license.
