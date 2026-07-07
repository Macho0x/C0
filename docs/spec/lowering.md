# C0 Lowering to Go

This document describes how C0 constructs are translated into Go. The target is Go 1.22 or later.

## Identifiers

- C0 identifiers become title-cased for exported Go identifiers.
- Type variables are erased; generic functions are monomorphized or use Go generics where applicable.
- C0 keywords that conflict with Go keywords are escaped with a trailing underscore.

## Types

| C0 | Go |
|---|---|
| `int` | `int` |
| `int64` | `int64` |
| `float` | `float64` |
| `bool` | `bool` |
| `string` | `string` |
| `bytes` | `[]byte` |
| `unit` | `struct{}` |
| `'a list` | `[]T` (where T is the lowered element type) |
| `'a option` | generated `OptionT` struct |
| `('ok, 'err) result` | generated `ResultOkErr` struct |
| tuple | generated struct with positional fields |
| record | Go struct with exported fields |
| ADT | Go interface + one struct per variant |

## Expressions

| C0 | Go |
|---|---|
| `let x = e1 in e2` | `x := e1; e2` or `const x = e1; e2` |
| `if c then t else f` | `if c { t } else { f }` |
| `match e with \| Pi -> ei` | `switch v := e.(type) { case ... }` |
| `f x y` | `f(x, y)` for fully applied multi-arg functions |
| `fun x -> e` | `func(x T) U { return e }` |
| `x :: xs` | `append([]T{x}, xs...)` |
| `[a; b; c]` | `[]T{a, b, c}` |
| `{ x = 1; y = 2 }` | `Point{X: 1, Y: 2}` |
| `r.x` | `r.X` |
| `e \|> f` | `f(e)` |
| `e ?` | `tmp, err := e; if err != nil { return ... }` |

## Pattern guards

Guards lower to `if` statements inside the corresponding type-switch case.

## Option and Result

```c0
type 'a option = None | Some of 'a
```

→

```go
type OptionTag byte
const (OptionTagNone OptionTag = iota; OptionTagSome)

type OptionInt struct {
    tag  OptionTag
    some *int
}

func NewOptionIntNone() OptionInt { return OptionInt{tag: OptionTagNone} }
func NewOptionIntSome(v int) OptionInt { return OptionInt{tag: OptionTagSome, some: &v} }
```

Result follows the same pattern with `Ok` and `Error` constructors.

## Modules

A C0 module `Foo.Bar` lowers to Go package `bar` in directory `foo/bar`. Exported values are title-cased.

## Match exhaustiveness

The compiler proves exhaustiveness. The lowered Go type switch includes a `default` case that panics. This panic is unreachable for well-typed C0 programs but satisfies Go's requirement that a switch over an interface type be complete.

## Partial application

Partially applied functions lower to Go closures. Fully applied multi-parameter functions lower to direct multi-parameter Go functions.

## Effect rows

Effect rows are **erased** entirely. The `with { ... }` clause on a function type produces no Go code.

| C0 | Go |
|---|---|
| `f : T with { io }` | `func F(...) T` (no change) |
| `f : T with {}` | `func F(...) T` (no change) |
| `f : T` | `func F(...) T` (no change) |

Effect row unification is a compile-time-only check.

## Refinement contracts

`where` clauses lower to runtime `panic` guards.

| C0 | Go |
|---|---|
| `(x: T where P)` parameter | `if !(P) { panic("f: precondition violated: P") }` at function entry |
| `: U where Q` return type | `defer func() { if !(Q) { panic("f: postcondition violated: Q") } }()` with named return |

`it` in `P` is replaced with the parameter name. `result` in `Q` refers to the Go named return value.

```c0
let safeDiv (a: int) (b: int where b <> 0) : int = a / b
```

→

```go
func SafeDiv(a, b int) int {
    if !(b != 0) { panic("safeDiv: precondition violated: b <> 0") }
    return a / b
}
```

```c0
let clamp (x: int) (lo: int) (hi: int where hi >= lo) : int where result >= lo && result <= hi = ...
```

→

```go
func Clamp(x, lo, hi int) (ret0 int) {
    if !(hi >= lo) { panic("clamp: precondition violated: hi >= lo") }
    defer func() {
        if !(ret0 >= lo && ret0 <= hi) {
            panic("clamp: postcondition violated: result >= lo && result <= hi")
        }
    }()
    // ... body ...
    return
}
```

## Linear types

Linear types are **erased** in Go output.

| C0 | Go |
|---|---|
| `type handle : 1` | `type Handle = interface{}` or extern Go type |
| `f (h: handle) : unit` | `func F(h Handle) { ... }` |

Linear discharge checking is compile-time only. The emitted Go has no runtime linearity constraints.

## Region scopes

`region { ... }` computation expressions lower to inline Go with `defer Close(varName)` for each `let!`:

```c0
region {
    let! x = h
    do! useHandle x
    return ()
}
```

→

```go
{
    x := h
    defer Close(x)
    useHandle(x)
}
```

Each `let!` binding:
1. Binds the variable to the expression result.
2. Emits `defer Close(varName)` immediately after the binding.
3. The linear discharge checker marks the variable as discharged at the end of the region block.

## Source locations

Each generated Go construct is annotated with a comment or source-map entry mapping it back to the originating C0 source location.
