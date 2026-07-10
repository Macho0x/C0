# Goop Modules and Packages

## Module declaration

Every Goop source file begins with a module declaration:

```goop
module MyModule
```

A file-level module corresponds to a Go package. The module name determines the emitted Go package name and default file path.

## Nested modules (OCaml-style, minimal)

```goop
module Inner = struct
  let add (a: int) (b: int) : int = a + b
end

open Inner
```

Also supported (minimal implementations):

| Form | Role |
|------|------|
| `module M = struct … end` | Nested structure |
| `module type S = sig … end` | Signature |
| `module F (X : S) = struct … end` / `functor` | Functors |
| `open` / `include` | Bring names into scope |
| `let module M = … in …` | Local module |
| `.mli` | Interface file (`sig` items) |

See [14-ocaml-parity.md](14-ocaml-parity.md). Everyday projects still use one file-level `module` plus Go-style imports.

## Unified imports (Go-style)

Goop keeps Go-style import syntax (intentional extension). Legacy top-level-only `open` as the sole import mechanism and `extern "go"` were removed earlier.

```goop
module Main

import (
  golang "fmt"
  golang "strconv" { val Atoi : string -> (int, string) }
  goop "std.io"
  httpx golang "net/http"
  orderbook goop "github.com/you/app/orderbook"
)

import goop . "std.list"   (* dot import: unqualified exports *)
```

| Form | Meaning | `{ val … }` |
|------|---------|-------------|
| `import golang "path"` | Go package | Optional FFI signatures |
| `import goop "path"` | Goop module | Forbidden |
| `import goop . "path"` | Dot import | Forbidden |
| `alias golang "path"` | Go import with local alias | Optional |
| `alias goop "path"` | Qualified Goop import | Forbidden |

Logical paths like `"std.io"` resolve via `goop.toml` `[mappings]` or built-in defaults.

## Inline Go

```goop
@golang {
  func helper() int { return 42 }
}
val helper : unit -> int
```

## Visibility

```goop
private let helper x = x + 1
let main () = helper 1   (* OK in same module *)
```

`private type` brands ADT constructors at the module boundary (preferred over removed `newtype`).

## Configuration

See [11-package-manager.md](11-package-manager.md) for `goop get`, `goop.lock`, and `[dependencies]`.

## Compilation unit

One Go package is emitted per Goop module file. Transitive `import goop` is resolved for type-checking and test builds.
