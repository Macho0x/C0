# 17. Go interface implementation (`implements`)

**Status:** Shipped in Goop **1.3.0** (MVP).

## Motivation

Treelog and other slog-centric libraries need to define types that satisfy Go
interfaces such as `slog.Handler`. Today Goop can *call* Go via `import go`
and `@[go]`, but cannot emit Go method sets. OCaml-style `object … method … end`
lowers to Goop objects (`o#m`), not Go receivers — so it cannot satisfy
`slog.New`.

## Surface

### Go type imports

`import go` blocks accept `type Name` alongside `val`:

```goop
import go "fmt" {
  type Stringer
}

import go "log/slog" {
  type Handler
  type Attr
  type Record
  type Logger
  type Level
  val New : Handler -> Logger
  val String : string -> string -> Attr
}
```

`type Name` binds an **opaque Go named type** from that package (struct or
interface). Interfaces may appear in `implements`; values of opaque types are
passed through to Go without field projection in v1.

### `implements`

```goop
type point = { x : int; y : int }

implements Stringer for point with
  let String (p : point) : string = int_to_string p.x ^ "," ^ int_to_string p.y
end
```

Rules:

1. First parameter is the **receiver** (value or conceptually pointer).
2. Remaining parameters and return type must match the Go interface method.
3. Method names are case-sensitive and must match the Go interface.
4. Codegen emits pointer receivers: `func (p *point) String() string`.
5. Codegen emits `var _ fmt.Stringer = (*point)(nil)`.

### FFI supporting types (1.3.0 MVP)

| Goop | Go |
|------|-----|
| `error` | `error` |
| `'a ptr` | `*T` |
| `null` | `nil` |
| `is_null e` | `e == nil` |
| `ptr_of e` | `&e` |
| `'a go_slice` | `[]T` |
| `go_slice_len` / `go_slice_append` / `go_slice_of_list` / `list_of_go_slice` | `len` / `append` / convert |
| Variadic params `...T` and `spread xs` | `...T` / `xs...` |
| Goop `T -> U` where Go expects `func(T) U` | matching `func` literal |

**Non-goals for 1.3.0:** `defer` keyword; bare `import go "pkg"` auto-export
discovery; OCaml objects satisfying Go interfaces; rewriting all of stdlib off
`@[go]`.

Mutex usage: `import go "sync"` + explicit `Lock` / `Unlock`.

## Lowering

```
implements I for T with
  let M (self : T) (a : A) : R = body
end
```

→

```go
func (self *T) M(a A) R {
  // body
}
var _ pkg.I = (*T)(nil)
```

Record types used as implementors lower as today (exported struct fields).
Opaque Go types in signatures lower to their qualified Go names
(`slog.Handler`, `fmt.Stringer`, …).

## Type checking

1. `I` must resolve to an imported Go interface type (or a known builtin
   interface alias from `import go`).
2. Each required method must be present with matching arity and types
   (structural check; optional `gosig` refinement when packages load).
3. Missing / wrong methods → typed errors (`FFI-IMPL001`, …).

## Relation to `@[go]`

`@[go]` remains for opaque Go bodies and escape hatches. Implementing a Go
interface no longer *requires* `@[go]` when the method bodies are expressible
in Goop.

See also: [15-lang-embeds.md](15-lang-embeds.md), [04-go-lowering.md](04-go-lowering.md).
