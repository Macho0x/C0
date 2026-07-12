# Goop Go Lowering Strategy

## Core rule

For **non-effectful** code, every Goop construct must lower to Go that a senior Go engineer would consider idiomatic.

**Exception:** OCaml 5-style resumptive effect handlers lower via CPS / free-monad. That output is not idiomatic hand-written Go; it is an explicit, documented exception (see [01-overview.md](01-overview.md)).

## Sum types

ADTs → one Go interface + one struct per variant (`go/ast` pattern).

```goop
type shape =
  | Circle of { radius: float }
  | Rect of { width: float; height: float }
```

→ `Shape` interface, `ShapeCircle` / `ShapeRect` structs, `NewShape…` constructors.

## Pattern matching

`match` → Go type switch. Exhaustiveness is checked in Goop; a defensive `panic("unreachable…")` default satisfies the Go compiler.

## Records

Records → Go structs. `{ p with x = 3.0 }` → struct literal reconstruction.

## `option` and `result`

Built-in generics → concrete structs + constructors (`NewOption…`, `NewResult…`). Prefer `match` in Goop source (no `?` operator).

## Refs

```goop
let r = ref 0 in
r := !r + 1
```

→

```go
r := new(int); *r = 0
*r = *r + 1
```

`ref e` allocates a pointer; `!r` is dereference; `r := e` is store. Mutable record fields lower to ordinary Go struct fields.

## Exceptions

```goop
try body with | Fail msg -> handle msg
```

→ Go `defer` + `recover` around `body`, matching arms on the recovered value. `raise e` / `failwith msg` → `panic(...)`.

```goop
try body finally cleanup
```

→ `defer cleanup` then `body`.

## Effect handlers (CPS)

`perform` and effect-handler `match` arms are rewritten by `internal/effects` into a minimal CPS / free-monad form (`__goop_perform`, `__goop_handle`) before codegen. Pure functions without `perform` stay direct-style.

## Functions and currying

Fully applied multi-arg functions → multi-parameter Go funcs. Partial application → closures.

Capitalized lets (`Add 2 3`) are parsed with an embedded first constructor
argument; codegen flattens that to `Add(2, 3)`. See
[19-call-lowering.md](19-call-lowering.md).

`unit` parameters are erased from Go signatures. Calls of the form `f ()`
omit the argument (including qualified `M.f ()` and externs like `time.Now ()`).

`let x = if c then a else b` lowers to an immediately invoked function so the
`if` can appear in expression position.

## Modules

A Goop module maps to a Go package (name lowercased; exports title-cased). Nested `struct` modules flatten or nest as packages according to the compiler’s module pass (minimal).

## Imports

```goop
import (
  go "fmt"
  goop "std.io"
)
```

→ Go `import` paths. Logical Goop paths resolve via `goop.toml` `[mappings]`.

## Go interface FFI

`import go "pkg" { type Interface }` imports an opaque Go named type.
`implements Interface for T with … end` emits pointer-receiver methods for
`*T` plus a Go compile-time interface assertion. FFI types lower directly:
`'a ptr` to `*T` (`ptr_of` takes an address and `null` is `nil`), `error` to
Go `error`, and `'a go_slice` to `[]T`. The `go_slice_*` helpers operate on
those Go slices; list conversion helpers are runtime identities because lists
already lower to slices. See [17-go-implements.md](17-go-implements.md).

## Go method and field FFI

An `import go` signature block can also declare selectors:

```goop
import go "bytes" {
  type Buffer
  val (b : Buffer ptr).String : unit -> string
}
```

Use `T ptr` when Go methods take pointer receivers (e.g. `*bytes.Buffer`).
Opaque `type Buffer` alone is the Go value type.

`val (x : T).M : A -> B` lowers to `x.M(a)`. A non-arrow declaration such as
`val (a : Attr).Key : string` lowers to the Go field selector `a.Key`.
Callbacks and `go_slice` values remain ordinary Go function and slice values,
so `Record.Attrs r (fun a -> true)` lowers to `r.Attrs(func(a slog.Attr) bool
{ return true })`. See [18-go-methods.md](18-go-methods.md).

## Tuples and lists

Tuples → generated positional structs. `'a list` → `[]T` with `append` for cons.

## Arrays

`'a array` → `[]T`.

| Goop | Go |
|---|---|
| `Array.make n default` | `make([]T, n)` + init loop |
| `Array.length arr` | `len(arr)` |
| `arr.(i)` | `arr[i]` |
| `arr.(i) <- v` | `arr[i] = v` |

## Loops and sequencing

`for i = lo to hi do body done` → inclusive Go `for`.  
`while e do body done` → `for e { body }`.  
`begin … end` → block or IIFE in expression position.

## Qualified constructors

`Type.Ctor` is a parse-time disambiguator; same Go constructor as unqualified `Ctor`.

## Refinement contracts

`where` clauses → runtime `panic` guards when unproven. Proven VCs (interval solver or optional Z3) may skip call-site guards; exported entries keep entry guards.

```go
func SafeDiv(a, b int) int {
    if !(b != 0) {
        panic("safeDiv: precondition violated: b <> 0")
    }
    return a / b
}
```

## Linear types

Erased to `interface{}` or the extern Go type. Discharge checking is compile-time only.

## Resource cleanup

Prefer `try … finally` or explicit `Close` hand-off. Legacy `region { }` / `using` CEs are removed (PARSE-MIG013).

## Inline Go (`@[go]`) / C (`@[c]`)

```goop
import (
  go "net/http"
  go "io"
)

@[go] {
  func httpGetString(url string) string { ... }
}
val httpGetString : string -> string
```

`@[go]` is emitted verbatim into the generated package. Names must match `val` bindings. Unit args are elided.

`@[c]` concatenates into a cgo preamble + `import "C"` with auto-wrappers for primitive `val` types. See [15-lang-embeds.md](15-lang-embeds.md).

## Source maps

Stack traces and diagnostics map back to `.goop` locations.
