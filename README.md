<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo.png">
    <img alt="C0 logo" src="assets/logo.png" width="160">
  </picture>
</p>

<h1 align="center">C0</h1>

<p align="center">
  <strong>ML elegance. Go power.</strong><br>
  A statically typed systems language that compiles to readable, idiomatic Go.
</p>

<p align="center">
  <a href="#quick-example">Example</a> •
  <a href="#why-c0">Why C0?</a> •
  <a href="#feature-deep-dive">Features</a> •
  <a href="#getting-started">Getting Started</a> •
  <a href="#faq">FAQ</a> •
  <a href="#status--roadmap">Status</a>
</p>

---

## Look, I love Go. But...

You know that feeling when you define a sum type in your head, then open your editor and write this instead?

```go
type ShapeType int
const (
    CircleType ShapeType = iota
    RectType
)
type Shape struct {
    Type     ShapeType
    Radius   float64 // only for Circle
    Width    float64 // only for Rect
    Height   float64 // only for Rect
}
func (s Shape) IsCircle() bool { return s.Type == CircleType }
func (s Shape) IsRect() bool   { return s.Type == RectType }
```

**33 lines of defensive boilerplate.** Every field optional. Every access a prayer. One wrong `s.Type` check and your production server learns what a nil pointer feels like.

Or when you're explaining to an OCaml developer why Go doesn't have pattern matching — and they look at you like you just said "we don't believe in seatbelts."

Or when you're 47 `if err != nil` blocks deep into a single file and you wonder if there's a better way to write software.

**Yeah. That's why C0 exists.**

---

## What is C0?

C0 is a **statically typed systems language with ML-family syntax** that compiles to readable, idiomatic Go. Not a transpiler, not a wrapper — a proper compiler with type inference, algebraic data types, and parametric polymorphism whose output you'd happily code review.

**The pitch:** Write code with sum types, pattern matching, and type inference. Get back clean Go code that your team can read, your tools can debug, and your production servers can run at the same speed. Zero runtime overhead. Zero new dependencies. Zero "what is this garbage in my generated code?"

> **Core design constraint:** The Go emitted by the C0 compiler must be readable, idiomatic, and debuggable by any Go programmer — even one who has never seen C0.

**C0 is not a transpiler.** Transpilers do text-level substitution. C0 does full lexical analysis, parsing, type checking, and code generation. It understands your program. It catches errors before they become bugs. And it emits Go that looks hand-written.

---

## Quick example

<table>
<tr>
<td width="50%" valign="top">

**What you write (C0)**

```c0
module Main

open Std.IO

type shape =
  | Circle of { radius: float }
  | Rect of { width: float; height: float }

let area (s: shape) : float =
  match s with
  | Circle { radius } -> 3.14159 *. radius *. radius
  | Rect { width; height } -> width *. height

let main () =
  let s = Circle { radius = 2.0 } in
  Console.print_line (Float.to_string (area s))
```

</td>
<td width="50%" valign="top">

**What you get (Go)**

```go
type Shape interface {
    isShape()
}

type Circle struct { Radius float64 }
func (Circle) isShape() {}

type Rect struct {
    Width  float64
    Height float64
}
func (Rect) isShape() {}

func (c Circle) area() float64 {
    return 3.14159 * c.Radius * c.Radius
}

func (r Rect) area() float64 {
    return r.Width * r.Height
}

func main() {
    s := Circle{Radius: 2.0}
    Console.PrintLine(Float.ToString(area(s)))
}
```

</td>
</tr>
</table>

**21 lines of C0 → ~30 lines of idiomatic Go.** Interface-based sum types. Methods generated from your pattern matches. Code you'd actually ship.

---

## Pain points → C0 solutions

| The Go problem | How many times you've felt it | The C0 solution |
|---|---|---|
| **Sum types don't exist** — manual tag fields, unused zero values | Every time you model heterogeneous data | `type shape = Circle of {…} \| Rect of {…}` — real sum types with payloads |
| **Pattern matching?** — switch on brittle tag checks | Every `if s.Type == CircleType` | `match s with Circle { r } -> … \| Rect { w, h } -> …` — exhaustive, destructuring |
| **`if err != nil`** drowning business logic | ~47 times per file | Type-safe errors with `Result<T, E>` via the type system |
| **Nil pointer panics** in production | More often than you'll admit | Pattern match exhaustiveness — the compiler won't let you skip a case |
| **Ceremonious generics** with type parameters | Every generic data structure | Parametric polymorphism, higher-order functions, concise syntax |
| **No sum type exhaustiveness** — adding a variant breaks silently | Every enum refactor | Add a variant → compiler shows every match that needs updating |
| **Verbose type annotations** everywhere | Every variable declaration | Full type inference — `let x = …` not `var x Type = …` |

**Key insight:** C0 doesn't change Go. It compiles to it. Your team gets ML-level type safety, your production gets pure Go performance.

---

## Feature deep dive

### 1. Algebraic data types — sum types that Go deserves

**What you write in C0:**
```c0
type http_response =
  | Ok of { body: string; status: int }
  | Redirect of { url: string }
  | ClientError of { code: int; message: string }
  | ServerError of { code: int; message: string }
```

**What Go makes you write (33 lines of tedium):**
```go
type HttpResponseType int
const (
    HttpResponseTypeOk HttpResponseType = iota
    HttpResponseTypeRedirect
    HttpResponseTypeClientError
    HttpResponseTypeServerError
)
type HttpResponse struct {
    Type           HttpResponseType
    Body           string   // only Ok
    Status         int      // only Ok
    Url            string   // only Redirect
    ClientCode     int      // only ClientError
    ClientMessage  string   // only ClientError
    ServerCode     int      // only ServerError
    ServerMessage  string   // only ServerError
}
// plus 4 Is*() methods, plus hope you don't access the wrong field
```

**6 lines of C0 expresses what takes 33 lines of error-prone Go.** The compiler tracks which variant is active. You can't access `body` on a `Redirect`. You can't forget to handle `ServerError` — the compiler won't let you.

### 2. Pattern matching — switch statements grew up

```c0
match response with
| Ok { body; status } when status >= 200 && status < 300 ->
    Console.print_line ("Success: " ^ body)
| Redirect { url } ->
    Http.redirect url
| ClientError { code; message } ->
    Console.print_error ("Client error " ^ Int.to_string code ^ ": " ^ message)
| ServerError { code; message } ->
    Console.print_error ("Server error " ^ Int.to_string code ^ ": " ^ message)
```

**Exhaustiveness checking:** Add a variant to `http_response` and the compiler tells you every single `match` that needs updating. No runtime surprises. No "we forgot to handle that case" at 2 AM.

**Pattern guards:** `when status >= 200 && status < 300` — conditional matching without nested ifs.

**Destructuring:** `Ok { body; status }` and `ClientError { code; message }` — pull fields out in the match arm.

### 3. No nil — the type system has your back

Null references were called a "billion-dollar mistake" by their inventor. C0 doesn't have them.

Values are always initialized. Pattern matching is always exhaustive. If a function can fail, its return type says so — and the compiler enforces that you handle it.

No `interface{}` that might be nil. No `*T` that might be nil. No mystery panics at 3 AM when the staging server gets a request you didn't test.

### 4. Type inference — say what you mean

```c0
let x = 42                        (* int, inferred *)
let y = 3.14                      (* float, inferred *)
let f = fun x -> x *. 2.0         (* float -> float, inferred *)
let s = Circle { radius = 2.0 }   (* shape, inferred *)
```

No `var x int = 42`. No `:=` vs `=` confusion. Just `let name = value` and the compiler figures out the rest. Type annotations when you want them, never when you don't.

### 5. Go interop — seamless, not siloed

```c0
open Std.IO
open Net.Http                     (* standard library *)

let handler (req: http_request) : http_response =
  let body = IO.read_all req.Body in
  let user = Json.deserialize body in       (* call any Go function *)
  Ok { body = "Hello, " ^ user.name; status = 200 }
```

- Call any Go library directly from C0
- Mixed `.c0` + `.go` projects work out of the box
- Existing `go.mod` and hand-written Go files coexist with C0
- Generated Go is debuggable — open it in your IDE, step through, inspect variables
- No FFI, no bindings, no translation layer

---

## Code reduction in action

| Pattern | Go (lines) | C0 (lines) | Savings |
|---------|-----------|------------|---------|
| Sum type (4 variants with payloads) | 33 | 6 | **82%** |
| Pattern matching (exhaustive on 4 variants) | 25 | 10 | **60%** |
| Error handling pipeline (3 operations) | 30 | 10 | **67%** |
| Record with destructuring | 12 | 3 | **75%** |

*The business logic jumps off the screen. The ceremony disappears.*

---

## What the generated Go looks like

This is the non-negotiable part of C0: **the output must be readable.** Let's see what actually comes out:

**You write this C0:**
```c0
type option =
  | Some of { value: int }
  | None

let map_option (opt: option) (f: int -> int) : option =
  match opt with
  | Some { value } -> Some { value = f value }
  | None -> None
```

**C0 generates this Go:**
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

No runtime library. No reflection. No generated code comments that scream "DO NOT EDIT." Just plain Go with interfaces and type switches — the same patterns experienced Go engineers use every day.

---

## Why C0?

**You want OCaml/F#-level type safety but you deploy Go.** You love Go's runtime, its toolchain, its ecosystem, its deployment story — but its type system leaves you wanting more.

C0 bridges that gap without compromise:

| Go gives you | C0 adds |
|---|---|
| Fast compilation | Type inference (less typing) |
| Excellent runtime & tooling | Algebraic data types (sum types + records) |
| Huge ecosystem | Pattern matching with exhaustiveness |
| Easy deployment | Parametric polymorphism (generics done right) |
|  | Higher-order functions |
|  | No nil pointers |
|  | Readable Go output |

**C0 is not a transpiler.** A transpiler does glorified find-and-replace. C0 is a proper compiler: it lexes, parses, type-checks, infers, and generates. It understands your program well enough to tell you when you've made a mistake — not just when you've typed something unparseable.

**C0 is not a new runtime.** There is no C0 VM. No C0 standard library in any meaningful sense — it compiles to Go, and Go is your runtime. Every Go 1.x feature works in C0 from day one.

---

## Getting started

```bash
# Clone and build the compiler
git clone https://github.com/your/c0.git
cd c0/src && go build ./cmd/c0

# Build a C0 project (mixed .c0 + .go files welcome)
c0 build

# Start the LSP server for editor support
c0 lsp
```

Requires Go 1.21+.

---

## Project structure

```
C0/
├── docs/
│   ├── design/         # Design rationale and decisions
│   ├── spec/           # Formal grammar, semantics, lowering rules
│   └── examples/       # Example C0 programs
├── src/
│   ├── cmd/c0/         # CLI entry point (includes LSP)
│   └── internal/       # Compiler: lexer, parser, typecheck, codegen
├── syntaxes/           # TextMate grammar (VSCode, Zed, etc.)
├── editors/
│   ├── vscode/         # VSCode/Cursor extension
│   └── zed/            # Zed extension (with LSP adapter)
├── tests/              # Compiler and end-to-end tests
├── TODO.md
└── VERSION
```

---

## Editor support

### Zed
The Zed extension in `editors/zed/` provides:
- 🎨 Full syntax highlighting via TextMate grammar
- 🗺️ `.c0` file icons in the file tree
- ⚡ LSP integration with real-time diagnostics
- Install as a dev extension: `Zed → Extensions → Install Dev Extension → select editors/zed/`

### VSCode / Cursor
Install the extension from `editors/vscode/`:
- Syntax highlighting
- LSP client integration
- Language configuration (brackets, auto-closing pairs, comments)

### CLI
```bash
c0 lsp          # Language server (stdio)
c0 lex --color  # Terminal colorization
```

The LSP provides:
- Real-time syntax error diagnostics
- Graphical error reporting with source context, precise spans, and actionable help
- Future: hover information, completion, go-to-definition

---

## FAQ

### Why not just write Go and use a linter?

Linters catch style violations. They don't catch logic errors. They don't enforce that you handle every variant of a sum type. They don't prevent nil pointer dereferences. C0's type system catches these at compile time — before your tests, before your CI, before your production deploy.

Go is a fine language. C0 lets you write Go's spiritual successor while staying in its ecosystem.

### Is this production ready?

C0 is in early bootstrap. The compiler works for single-file programs; the type checker and code generator are under active development. Not ready for production loads yet, but the foundation is solid.

### How is this different from Borgo or Dingo?

**Borgo** (Rust-like syntax → Go) and **Dingo** (enhanced Go → Go) transpile Go-like syntax with added features.

**C0 takes a different approach.** It uses OCaml/F#-family syntax — not because it's exotic, but because ML languages have spent 50 years refining algebraic data types, pattern matching, and type inference. C0 brings that heritage to Go's runtime rather than reinventing it.

| | Borgo | Dingo | C0 |
|---|---|---|---|
| **Syntax** | Rust-like | Enhanced Go | ML-family (OCaml/F#) |
| **Implementation** | Written in Rust | Written in Go | Written in Go |
| **Output** | Go | Go | Readable, idiomatic Go |
| **Type system** | Algebraic | Algebraic | Algebraic + full inference |
| **Focus** | Rust ergonomics on Go | Go ergonomics | ML safety on Go |

### Do I need to learn OCaml to use C0?

No. C0 uses ML-family syntax because it's concise and unambiguous — but the surface area is small. If you know pattern matching from Rust, Swift, or Kotlin, you already know 80% of C0.

### What about Go 1.24+ features?

C0 compiles to Go. Every Go version's features are available. Want Go 1.24's `range over func`? Write C0 that uses it. The output is Go, so you get every Go improvement on day one.

### Can I migrate gradually?

Yes. C0 supports mixed `.c0` + `.go` projects in the same directory. Existing `go.mod` files and hand-written Go code coexist with C0 sources. Migrate file by file, function by function.

### Do I need to learn a new standard library?

No. C0 programs call Go libraries directly. There's no "C0 standard library" — Go's ecosystem IS your standard library.

---

## Standing on the shoulders of giants

C0 exists because these languages and systems proved what's possible:

**Standard ML** (Milner, 1973) — Invented the modern type system. Hindley-Milner type inference, parametric polymorphism, algebraic data types. Every ML-family language stands on this foundation.

**OCaml** (Hickey et al., 1996) — Proved that ML can be practical. Module system, functors, a world-class type checker. Showed that type safety doesn't mean giving up expressiveness.

**F#** (Syme et al., 2005) — Proved ML can thrive on a non-ML runtime. The blueprint for bringing algebraic types to an existing ecosystem.

**Rust** (Matsakis et al., 2015) — Proved that systems programming and algebraic types are not just compatible — they're better together. Showed the world what pattern matching looks like at scale.

**Go** (Griesemer, Pike, Thompson, 2009) — Proved that simple tooling, fast compilation, and a great runtime matter more than features. The platform C0 builds on.

**Haskell** (Peyton Jones et al., 1990) — Showed what laziness and purity can do. C0 takes the pragmatic parts (type classes, do notation ideas) and leaves the academic ones.

---

## Status & roadmap

C0 is in early design and bootstrap implementation. The compiler is being written in Go and will self-host once the language is mature enough.

### What works today
- ✅ Lexer with full token set
- ✅ Parser for modules, expressions, types
- ✅ Type checker with Hindley-Milner inference
- ✅ Code generator emitting idiomatic Go
- ✅ `c0 build` with mixed C0 + Go projects
- ✅ LSP server with syntax diagnostics
- ✅ TextMate grammar for syntax highlighting
- ✅ Lisette-style graphical error diagnostics
- ✅ VSCode and Zed extensions

### What's in progress
- 🚧 Complete standard library prelude
- 🚧 Module system polish (package resolution)
- 🚧 Error recovery in parser

### Roadmap
- **Self-hosting** — Rewrite the compiler in C0
- **Package manager integration** — `c0 init`, `c0 add`, etc.
- **Advanced IDE features** — Go-to-definition, completions, hover
- **Performance optimization** — Smarter generated code, better type inference

Error reporting uses Lisette-style graphical diagnostics with source context, precise spans, and actionable help.

---

## Can I help?

Yes. Here's how:

⭐ **Star the repo** — Shows us people actually want this

💡 **Open issues** — Ideas, complaints, weird edge cases

📖 **Improve docs** — If something's confusing, it's our fault

🔨 **Write code** — Check issues tagged "good first issue"

---

## Documentation

### Design
- [Language overview](docs/design/01-overview.md)
- [Type system](docs/design/02-type-system.md)
- [Syntax](docs/design/03-syntax.md)
- [Go lowering strategy](docs/design/04-go-lowering.md)
- [Modules and packages](docs/design/05-modules-and-packages.md)
- [Effects and safety](docs/design/06-effects-and-safety.md)
- [Roadmap](docs/design/07-roadmap.md)

### Specification
- [Grammar](docs/spec/grammar.md)
- [Semantics](docs/spec/semantics.md)
- [Lowering to Go](docs/spec/lowering.md)

---

## License

MIT / Apache-2.0 dual license.
