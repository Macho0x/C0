# Goop Go Lowering Strategy

## Core rule

Every Goop construct must lower to Go that a senior Go engineer would consider idiomatic. The output is not an internal IR; it is the artifact other programmers will read, debug, and import.

## Sum types

Goop ADTs lower to **one Go interface plus one struct per variant**, following the pattern used by `go/ast`, `errors`, and `encoding/json`.

```goop
type shape =
  | Circle of { radius: float }
  | Rect of { width: float; height: float }
```

→

```go
type Shape interface { isShape() }

type ShapeCircle struct { Radius float64 }
func (ShapeCircle) isShape() {}

type ShapeRect struct { Width, Height float64 }
func (ShapeRect) isShape() {}

func NewShapeCircle(radius float64) Shape { return ShapeCircle{Radius: radius} }
func NewShapeRect(width, height float64) Shape { return ShapeRect{Width: width, Height: height} }
```

This is preferred over tagged structs because:

1. It matches existing Go conventions.
2. Only the active variant's data exists in memory.
3. Type switches give safe access to variant fields.
4. It avoids `O(n²)` `Is*()` method bloat.

## Pattern matching

`match` lowers to a Go type switch:

```goop
let area (s: shape) : float =
  match s with
  | Circle { radius } -> pi *. radius *. radius
  | Rect { width; height } -> width *. height
```

→

```go
func area(s Shape) float64 {
    switch v := s.(type) {
    case ShapeCircle:
        return math.Pi * v.Radius * v.Radius
    case ShapeRect:
        return v.Width * v.Height
    }
    panic("unreachable: unhandled Shape variant")
}
```

The `panic` is unreachable because the Goop compiler verifies exhaustiveness. It is emitted as a defensive default so the Go compiler is happy.

## Records

Records lower to Go structs with exported fields:

```goop
type point = { x: float; y: float }
let p = { x = 1.0; y = 2.0 }
```

→

```go
type Point struct { X, Y float64 }

var p = Point{X: 1.0, Y: 2.0}
```

Record update `{ p with x = 3.0 }` lowers to struct literal reconstruction.

## `option` and `result`

Built-in generic types lower to Go structs with constructor functions. Method names match Go conventions (`MustSome`, `SomeOr`, `MustOk`, `OkOr`).

```goop
let valueOrDefault (opt: int option) (default: int) : int =
  match opt with
  | Some x -> x
  | None -> default
```

→

```go
func valueOrDefault(opt OptionInt, def int) int {
    if opt.tag == OptionTagSome { return *opt.some }
    return def
}
```

For exported top-level functions, Goop emits per-instantiation concrete types because Go does not support type constructors at runtime.

## Error propagation

The `?` operator lowers to the standard Go idiom:

```goop
let readConfig (path: string) : result<config, error> =
  let bytes = File.readAllBytes path ?
  ...
```

→

```go
func readConfig(path string) ResultConfigError {
    tmp, err := File_readAllBytes(path)
    if err != nil {
        return ResultConfigError{tag: ResultTagError, err: &err}
    }
    bytes := tmp
    ...
}
```

## Functions and currying

Curried Goop functions lower to multi-parameter Go functions when fully applied, or to closures when partially applied.

```goop
let add (x: int) (y: int) : int = x + y
let addFive = add 5
let seven = addFive 2
```

→

```go
func add(x, y int) int { return x + y }

var addFive = func(y int) int { return add(5, y) }

var seven = addFive(2)
```

The compiler optimizes known fully-applied calls to direct calls.

## Modules

A Goop module maps to a Go package. Module names are lowercased for package names; exported declarations are title-cased.

```goop
module Math

let pi = 3.14159
let add (x: int) (y: int) : int = x + y
```

→

```go
package math

const Pi = 3.14159

func Add(x, y int) int { return x + y }
```

## Imports

`open Std.IO` resolves to a Go import path. The compiler maintains a mapping from Goop module names to Go import paths.

```goop
open Std.IO
```

→

```go
import "goop.dev/std/io"
```

## Tuples

Tuples lower to generated structs with positional fields:

```goop
let p : int * string = (42, "hello")
```

→

```go
type Tuple2IntString struct { F0 int; F1 string }

var p = Tuple2IntString{F0: 42, F1: "hello"}
```

## Lists

The built-in `'a list` lowers to a Go slice `[]T`. List constructors compile to slice operations:

```goop
let xs = [1; 2; 3]
let ys = 0 :: xs
```

→

```go
var xs = []int{1, 2, 3}
var ys = append([]int{0}, xs...)
```

## Mutability

`mutable` bindings lower to Go `var`:

```goop
let mutable counter = 0
counter <- counter + 1
```

→

```go
var counter = 0
counter = counter + 1
```

Immutable `let` lowers to `const` when possible and `var` otherwise.

## Effect rows

Effect rows are **completely erased** in Go output. No Go code is emitted for `with { ... }` clauses. The compiler validates effect usage at compile time; the lowered Go is identical to what would be emitted for a function without effect annotations.

```goop
let readFile (path: string) : string with { io } = ...
```

→

```go
func ReadFile(path string) string { ... }
```

This is a zero-cost abstraction: full compile-time effect tracking with no runtime representation.

## Refinement contracts

`where` clauses lower to runtime `panic` guards. Preconditions (on parameter types) emit an `if` check at function entry. Postconditions (on the return type) emit a `defer` check using Go named return values.

```goop
let safeDiv (a: int) (b: int where b <> 0) : int = a / b
```

→

```go
func SafeDiv(a, b int) int {
    if !(b != 0) {
        panic("safeDiv: precondition violated: b <> 0")
    }
    return a / b
}
```

Return refinement:

```goop
let clamp (x: int) (lo: int) (hi: int where hi >= lo) : int where result >= lo && result <= hi = ...
```

→

```go
func Clamp(x, lo, hi int) (ret0 int) {
    if !(hi >= lo) {
        panic("clamp: precondition violated: hi >= lo")
    }
    defer func() {
        if !(ret0 >= lo && ret0 <= hi) {
            panic("clamp: postcondition violated: result >= lo && result <= hi")
        }
    }()
    // ... function body ...
    return
}
```

There is no SMT solver; all refinements are checked at runtime.

## Linear types

Linear types are **erased** in Go output. They lower to `interface{}` or the extern-declared Go type:

```goop
type handle : 1
let f (h: handle) : unit = ...
```

→

```go
type Handle = interface{}
func F(h Handle) { ... }
```

The linearity discipline (discharge checking, double-use rejection) is enforced at compile time and imposes zero runtime cost.

## Region scopes

`region { ... }` computation expressions lower to inline Go with `defer Close(varName)` for each `let!` binding:

```goop
let process (h: handle) : unit =
  region {
    let! x = h
    do! useHandle x
    return ()
  }
```

→

```go
func Process(h Handle) {
    x := h
    defer Close(x)
    useHandle(x)
}
```

Each `let!` acquires a linear resource and registers a `defer Close()` at region exit. The linear discharge checker auto-discharges region-bound variables. This replaces the legacy `using` block with compile-time-guaranteed cleanup.

## Inline Go extern

Goop allows embedding raw Go functions directly inside `extern` declarations using a `go { ... }` block. The Go code is emitted verbatim into the generated file, enabling multi-step Go logic without editing the compiler.

```goop
extern "go" "net/http" {}
extern "go" "encoding/json" {}
extern "go" "io" {}
extern "go" "strconv" {}

extern "go" "" {
  go {
    func httpGetString(url string) string {
      resp, err := http.Get(url)
      if err != nil { return "" }
      defer resp.Body.Close()
      body, _ := io.ReadAll(resp.Body)
      return string(body)
    }
  }
  val httpGetString : string -> string
}
```

→

```go
import (
    "net/http"
    "encoding/json"
    "io"
    "strconv"
)

func httpGetString(url string) string {
    resp, err := http.Get(url)
    if err != nil { return "" }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    return string(body)
}

func main() {
    data := httpGetString("https://...")
    // ...
}
```

Key rules:

1. **Imports**: Declare required Go packages with separate `extern "go" "path" {}` blocks (empty val/go lists). The compiler adds these to the Go import block.
2. **Same-package helpers**: Use `extern "go" "" { go { ... } }` for helpers that need no extra import. The Go code is emitted directly into the package.
3. **Names must match**: The Go function name in the `go { ... }` block must match the Goop name declared in `val name : type`. The Goop-to-Go name translation is identity for inline extern (no mangling).
4. **Unit arguments are elided**: Goop `()` calls become zero-argument Go calls (`nowString()` not `nowString(struct{}{})`).
5. **Any valid Go code works**: Functions, types, constants, variables — the entire `go { ... }` block is emitted verbatim.

## Source maps

Goop emits Go source maps so that stack traces, debugger breakpoints, and error messages refer to `.goop` source locations rather than generated `.go` lines.
