<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo.png">
    <img alt="C0 logo" src="assets/logo.png" width="160">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/Macho0x/C0/actions/workflows/ci.yml">
    <img alt="CI status" src="https://github.com/Macho0x/C0/workflows/CI/badge.svg">
  </a>
  <a href="https://github.com/Macho0x/C0/releases">
    <img alt="Latest release" src="https://img.shields.io/github/v/release/Macho0x/C0?display_name=tag&label=release">
  </a>
  <img alt="License" src="https://img.shields.io/badge/license-MIT%2FApache--2.0-blue">
</p>

<h1 align="center">C0</h1>

<p align="center">
  <strong>OCaml's type system. Go's runtime. No compromises.</strong><br>
  A statically typed language with algebraic data types, pattern matching,<br>
  linear types, and compile-time race detection — that compiles to readable Go.
</p>

<p align="center">
  <a href="#examples">Examples</a> •
  <a href="#what-c0-catches-that-go-doesnt">Compile-time safety</a> •
  <a href="#side-by-side-with-go">Go comparison</a> •
  <a href="#getting-started">Getting Started</a> •
  <a href="#faq">FAQ</a>
</p>

---

## Examples

### Algebraic data types + pattern matching

```c0
module main

type Shape =
  | Circle of { radius: float }
  | Rect of { width: float; height: float }
  | Point

let area (s: Shape) : float =
  match s with
  | Circle { radius } -> 3.14159 *. radius *. radius
  | Rect { width; height } -> width *. height
  | Point -> 0.0

let main () =
  print_line (int_to_string (area (Circle { radius = 2.0 })))
```

**The compiler checks that every `match` is exhaustive.** Add a `Triangle` variant to `shape` and the compiler tells you every single `match` that needs updating. No runtime surprises. No "we forgot to handle that case" at 2 AM.

**The compiler catches dead code.** Switch the order so `Point` comes first with a wildcard `_` and you'll get an "unreachable pattern" error. Every pattern is checked for redundancy.

### Compare to Go: same semantics, no boilerplate

This C0 type:

```c0
type http_response =
  | Ok of { body: string; status: int }
  | Redirect of { url: string }
  | ClientError of { code: int; message: string }
  | ServerError of { code: int; message: string }
```

Expresses what Go makes you write in **33 lines** of manual tag fields, zero values, and hope:

```go
type HttpResponseType int
const (
    HttpResponseTypeOk HttpResponseType = iota
    HttpResponseTypeRedirect
    HttpResponseTypeClientError
    HttpResponseTypeServerError
)
type HttpResponse struct {
    Type          HttpResponseType
    Body          string  // only Ok — zero value when not Ok
    Status        int     // only Ok — zero value when not Ok
    Url           string  // only Redirect
    ClientCode    int     // only ClientError
    ClientMessage string  // only ClientError
    ServerCode    int     // only ServerError
    ServerMessage string  // only ServerError
}
// plus 4 Is*() methods
// plus hope you never access Body on a Redirect
```

**6 lines of C0. 33 lines of Go. Nothing is optional. Nothing can be accessed wrong.**

### Exhaustiveness checking prevents production bugs

```c0
match response with
| Ok { body; status } ->
    process_body body status
| ClientError { code; message } ->
    log_error code message
(* Whoops — forgot Redirect and ServerError! *)
```

**C0 won't compile this.** The compiler reports:

```
Error: Non-exhaustive pattern match
  --> examples/handler.c0:10:3
  |
5 |   match response with
  |         ^^^^^^^^
  |
  The following patterns are not covered:
  - Redirect { url }
  - ServerError { code; message }
```

In Go, this would compile silently. You'd find it in production when the first redirect response arrives.

### Recursive functions with let rec

```c0
let factorial (n: int) : int =
  let rec loop (acc: int) (m: int) : int =
    match m <= 1 with
    | true -> acc
    | false -> loop (acc * m) (m - 1)
  in
  loop 1 n

let main () =
  assert (factorial 5 = 120)
```

### Higher-order functions

```c0
let double (x: int) : int = x + x
let apply (f: int -> int) (x: int) : int = f x
let compose (f: int -> int) (g: int -> int) (x: int) : int = f (g x)

let main () =
  let chk1 = assert (apply double 5 = 10) in
  assert (compose double double 3 = 12)
```

### Records with field access

```c0
type point = { x: int; y: int }

let makePoint (x: int) (y: int) : point = { x = x; y = y }
let distance (p: point) (q: point) : bool =
  p.x = q.x && p.y = q.y

let main () =
  let p = makePoint 3 4 in
  let chk1 = assert (p.x = 3) in
  assert (p.y = 4)
```

### Pattern guards and conditional matching

```c0
type opt_int = | Some of int | None

let describe (x: opt_int) : string =
  match x with
  | Some n when n > 100 -> "big"
  | Some n when n > 0 -> "positive"
  | Some _ -> "zero or negative"
  | None -> "none"

let main () =
  let chk1 = assert (describe (Some 500) = "big") in
  assert (describe None = "none")
```

### Active patterns for custom matching logic

```c0
module ActivePatternTest

let (|Positive|_|) (n: int) : int option =
  if n > 0 then Some n else None

let describe (n: int) : string =
  match n with
  | Positive _ -> "positive"
  | _ -> "other"

let main () =
  assert (describe 5 = "positive")
```

### Channels and goroutines

```c0
module Main

let main () =
  let ch = Chan.make () in
  let _ = go (fun () -> Chan.send ch 42) in
  let v = Chan.recv ch in
  assert (v = 42)
```

### Computation expressions (result, region)

```c0
type parse_error = InvalidFormat of string | OutOfRange

let parseAndValidate (input: string) : (int * string) result =
  result {
    let! n = parse input;
    let! validated = validate n;
    return validated
  }
```

### Refinement types

```c0
let safeDiv (a: int) (b: int where b <> 0) : int =
  a / b
```

The compiler verifies (or inserts a runtime check) that `b` is never zero when `safeDiv` is called.

---

## What C0 catches that Go doesn't

C0's type system is not just "Go with nicer syntax." It catches entire classes of bugs that Go silently ships to production. Here's what the compiler finds before your code ever runs:

### 🚫 Data races at compile time

```c0
let main () =
  let mutable counter = 0 in
  let _ = go (fun () -> counter <- counter + 1) in
  counter <- counter + 1
```

**C0 rejects this.** The linear type checker detects that `mutable` variable `counter` is captured by a goroutine closure while still accessible in the spawning scope. Result:

```
Error: potential data race — mutable variable "counter" captured by
       goroutine is still accessible in spawning scope
  --> examples/race.c0:4:16
  |
4 |   let _ = go (fun () -> counter <- counter + 1) in
  |                ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  |                captured here
5 |   counter <- counter + 1
  |   ^^^^^^^^^^^^^^^^^^^^^^^
  |   also accessible here
```

Go's race detector only catches this at **runtime** — and only in tests that actually trigger the concurrent access. C0 catches it at **compile time**, always.

### 🚫 Non-exhaustive pattern matching

Add a variant to an ADT, miss updating a `match`, and C0 refuses to compile. In Go, this is silent — you find it when the production server panics on the unhandled case.

### 🚫 Nil pointer dereferences

No `nil`. No `null`. No billion-dollar mistake. Every value is always initialized. Every `match` is always exhaustive. The type `option` and `result` replace nullable pointers, and the compiler enforces that you handle both cases.

### 🚫 Unused results and resources

Linear types (`type handle : 1`) ensure resources are used exactly once. A channel handle can't be duplicated. A file descriptor can't be forgotten. The compiler tracks ownership and complains if you:
- Use a linear value twice
- Throw away a linear value without consuming it
- Close a channel while it's still in use

### 🚫 Unreachable patterns and dead code

```c0
match x with
| true -> "yes"
| false -> "no"
| true -> "wait, what?"
```

C0 rejects the third arm — `true` is unreachable after `false`. Dead code can't hide.

### 🚫 Refinement type violations

```c0
let divide (numerator: int) (denominator: int where denominator <> 0) : int =
  numerator / denominator
```

The compiler tracks that `denominator` is non-zero through branches, function calls, and assignments. Unproven refinements get runtime guards — the same way Rust inserts bounds checks, but documented in the type.

---

## Side-by-side with Go

| Capability | Go | C0 |
|---|---|---|
| **Sum types** | Manual tag fields with unused zero values | `type T = A of T1 \| B of T2` |
| **Pattern matching** | `switch` on brittle tag checks | `match` with destructuring, guards, exhaustiveness |
| **Nil safety** | Pointer semantics, `nil` everywhere | No nil — `option`/`result` types enforced by compiler |
| **Data race detection** | Runtime race detector (flaky, slow) | Compile-time linear type analysis |
| **Generics** | Type parameters (Go 1.18+) | Parametric polymorphism with full inference |
| **Variable syntax** | `var x T = v` / `x := v` | `let x = v` — types always inferred |
| **Error handling** | `if err != nil` everywhere | `result { ... }` computation expressions |
| **Resource tracking** | Manual or defer | Linear types — compiler tracks ownership |
| **Mutation control** | Everything mutable by default | `mutable` keyword — immutability is default |
| **Boilerplate per sum type** | ~8 lines overhead per variant | Zero — variants are just constructors |

---

## What the generated Go looks like

C0's core constraint: **the output must be readable by any Go programmer who has never seen C0.**

C0 → Go:

```c0
type option = | Some of { value: int } | None

let map_option (opt: option) (f: int -> int) : option =
  match opt with
  | Some { value } -> Some { value = f value }
  | None -> None
```

Generates:

```go
type Option interface { isOption() }
type OptionSome struct { Value int }
func (OptionSome) isOption() {}
type OptionNone struct{}
func (OptionNone) isOption() {}

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

Interface-based sum types. Go type switches for pattern matching. No runtime library. No reflection. No generated code comments telling you not to edit it. Just plain Go patterns that experienced engineers use every day.

---

## Code reduction

| Pattern | Go (lines) | C0 (lines) | Savings |
|---|---|---|---|
| Sum type with 4 payload variants | 33 | 6 | **82%** |
| Exhaustive pattern match on 4 variants | 25 | 10 | **60%** |
| Error handling pipeline (3 ops) | 30 | 10 | **67%** |
| Record with field access | 12 | 3 | **75%** |

---

## Getting started

```bash
# Build the compiler
cd src && go build ./cmd/c0

# Check a file parses and type-checks
./c0 check hello.c0

# Run tests
./c0 test

# Start the LSP server
./c0 lsp
```

---

## FAQ

### Is this production ready?

C0 is in early bootstrap. The compiler works for single-file programs with full type checking and Go code generation. Not ready for production loads yet, but the type system and code generator are under active development. What works today:

- ✅ Full lexer, parser, type checker with Hindley-Milner inference
- ✅ Code generator emitting idiomatic Go (interface sum types, type switches)
- ✅ Exhaustiveness checking on pattern matches
- ✅ Linear type analysis (data race detection, resource tracking)
- ✅ Computation expressions (`result { ... }`, `region { ... }`)
- ✅ Effects tracking, refinement types, active patterns
- ✅ Channels, goroutines, and owned channels
- ✅ Extern Go interop — call any Go library, mixed `.c0` + `.go` projects
- ✅ LSP server with real-time diagnostics
- ✅ VSCode and Zed extensions

### How is this different from Borgo or Dingo?

**Borgo** (Rust-like → Go) and **Dingo** (enhanced Go → Go) transpile with similar safety goals. C0 differs by using OCaml/F#-family syntax — not for novelty, but because ML languages have spent 50 years perfecting algebraic types, pattern matching, and type inference. C0 brings that heritage to Go rather than reinventing it.

The key differentiator: **C0 is a proper compiler, not a transpiler.** It does full lexical analysis, parsing, type inference, and code generation. It understands your program well enough to catch data races, unreachable patterns, and refinement violations at compile time — not just when tests happen to trigger them.

### Do I need to learn OCaml?

No. C0's syntax is small and regular. If you know pattern matching from Rust, Swift, or Kotlin, you already know C0.

### Can I migrate gradually?

Yes. C0 supports mixed `.c0` + `.go` projects in the same directory. Existing `go.mod` files and hand-written Go code coexist with C0 sources. Migrate file by file, function by function.

### Do I need to learn a new standard library?

No. C0 compiles to Go. Use `import golang "fmt" { val Println : string -> unit }` for Go bindings, or `import golang "fmt"` for import-only. Embed multi-step Go with `@golang { … }` — no compiler changes needed.

---

## Status

C0 is in bootstrap implementation (written in Go, targeting self-hosting).

- ✅ Lexer, parser, type checker, code generator
- ✅ LSP server, VSCode extension, Zed extension
- ✅ 7 passing end-to-end test files
- 🚧 More tests, standard library prelude, package resolution
- 🔮 Self-hosting — rewrite the compiler in C0

---

## Documentation

- [Language overview](docs/design/01-overview.md)
- [Type system](docs/design/02-type-system.md)
- [Syntax](docs/design/03-syntax.md)
- [Go lowering strategy](docs/design/04-go-lowering.md)
- [Effects and safety](docs/design/06-effects-and-safety.md)
- [Grammar](docs/spec/grammar.md)
- [Examples](docs/examples/)

---

## License

MIT / Apache-2.0 dual license.
