# 18. Go method and field imports

**Status:** Shipped in Goop **1.4.0**.

## Motivation

Goop 1.3.0 added `implements` so Goop types can *satisfy* Go interfaces.
Treelog and similar libraries also need to *call* methods and read fields on
imported Go types (`Record.Attrs`, `Attr.Key`, `Mutex.Lock`, `Logger.With`).

Opaque `type Record` imports alone are not enough: values cannot project
fields or invoke methods without `@[go]`.

## Surface

### Method and field vals

Inside `import go "path" { … }`:

```goop
import go "log/slog" {
  type Record
  type Attr
  type Value
  type Logger

  val New : Handler -> Logger
  val (r : Record).Attrs : (Attr -> bool) -> unit
  val (a : Attr).Key : string
  val (a : Attr).Value : Value
  val (v : Value).Resolve : unit -> Value
  val (v : Value).String : unit -> string
  val (l : Logger).Info : string -> ...any -> unit
  val (l : Logger).With : ...any -> Logger
}

import go "sync" {
  type Mutex
  val (m : Mutex).Lock : unit -> unit
  val (m : Mutex).Unlock : unit -> unit
}
```

| Form | Meaning |
|------|---------|
| `val Name : τ` | Free function (1.3) |
| `val (x : T).M : τ` | Method `M` on `T`; `τ` is the type *after* the receiver |
| `val (x : T).F : τ` | Field `F` on `T` when `τ` is not a function type starting a call pattern — **fields** are non-arrow types; **methods** have arrow (or `unit -> …`) types |

Disambiguation: if the declared type after `:` is a function type (`->`), the
binding is a **method**; otherwise it is a **field**.

### Calls

```goop
Record.Attrs r (fun a -> true)
r.Attrs (fun a -> true)          (* equivalent via field-select-then-app *)
let k = a.Key in
let s = (v.Resolve ()).String () in
Mutex.Lock mu; ...; Mutex.Unlock mu
log.With (spread args)
```

Lowering preserves Go selector spelling (`Key`, `Attrs`, `Resolve`).

### `any`, spread, `go_slice` indexing

```goop
(* builtin *)
type any
val any_of : 'a -> any
val go_slice_get : 'a go_slice -> int -> 'a

xs.(i)   (* when xs : 'a go_slice → go_slice_get xs i *)
spread xs
```

## Non-goals (1.4.0)

- `defer` keyword
- Auto-discovery of all exports from a bare `import go "pkg"`
- Changing OCaml `#method` object semantics

See also: [17-go-implements.md](17-go-implements.md), [15-lang-embeds.md](15-lang-embeds.md).
