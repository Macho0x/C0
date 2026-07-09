# Goop Modules and Packages

## Module declaration

Every Goop source file begins with a module declaration:

```goop
module MyModule
```

A module corresponds to a Go package. The module name determines the emitted Go package name and default file path.

## Unified imports (v0.3.0)

Goop uses Go-style import syntax. Legacy `open` and `extern "go"` were removed in v0.3.0.

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
| `import goop "path"` | Goop module (logical or canonical path) | Forbidden |
| `import goop . "path"` | Dot import (replaces `open`) | Forbidden |
| `alias golang "path"` | Go import with local alias | Optional |
| `alias goop "path"` | Qualified Goop import (`alias.Name`) | Forbidden |

Logical paths like `"std.io"` resolve via `goop.toml` `[mappings]` or built-in defaults.

## Inline Go

`@golang { … }` embeds Go source in the current file (unchanged from v0.2):

```goop
@golang {
  func helper() int { return 42 }
}
val helper : unit -> int
```

## Visibility

Top-level `private` hides bindings from other modules:

```goop
private let helper x = x + 1
let main () = helper 1   (* OK in same module *)
```

## Configuration

See [11-package-manager.md](11-package-manager.md) for `goop get`, `goop.lock`, and `[dependencies]`.

## Compilation unit

One Go package is emitted per Goop module file. Multi-file Goop packages are planned; v0.3 resolves transitive `import goop` for type-checking and test builds.
