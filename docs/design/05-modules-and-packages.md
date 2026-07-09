# C0 Modules and Packages

## Module declaration

Every C0 source file begins with a module declaration:

```c0
module MyModule
```

A module corresponds to a Go package. The module name determines the emitted Go package name and default file path.

## Unified imports (v0.3.0)

C0 uses Go-style import syntax. Legacy `open` and `extern "go"` were removed in v0.3.0.

```c0
module Main

import (
  golang "fmt"
  golang "strconv" { val Atoi : string -> (int, string) }
  c0 "std.io"
  httpx golang "net/http"
  orderbook c0 "github.com/you/app/orderbook"
)

import c0 . "std.list"   (* dot import: unqualified exports *)
```

| Form | Meaning | `{ val … }` |
|------|---------|-------------|
| `import golang "path"` | Go package | Optional FFI signatures |
| `import c0 "path"` | C0 module (logical or canonical path) | Forbidden |
| `import c0 . "path"` | Dot import (replaces `open`) | Forbidden |
| `alias golang "path"` | Go import with local alias | Optional |
| `alias c0 "path"` | Qualified C0 import (`alias.Name`) | Forbidden |

Logical paths like `"std.io"` resolve via `c0.toml` `[mappings]` or built-in defaults.

## Inline Go

`@golang { … }` embeds Go source in the current file (unchanged from v0.2):

```c0
@golang {
  func helper() int { return 42 }
}
val helper : unit -> int
```

## Visibility

Top-level `private` hides bindings from other modules:

```c0
private let helper x = x + 1
let main () = helper 1   (* OK in same module *)
```

## Configuration

See [11-package-manager.md](11-package-manager.md) for `c0 get`, `c0.lock`, and `[dependencies]`.

## Compilation unit

One Go package is emitted per C0 module file. Multi-file C0 packages are planned; v0.3 resolves transitive `import c0` for type-checking and test builds.
