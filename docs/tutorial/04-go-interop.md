# 4. Go interop

Goop’s primary standard library is **Go itself**. Use `import golang` for packages and `@golang` for inline Go code.

## Import Go packages

```goop
module main

import (
  golang "strings" {
    val ToUpper : string -> string
  }
  golang "fmt"
)
```

- **Signature block** — declare types for functions you call from Goop.
- **Import-only** — `golang "fmt"` with no block; pair with `@golang` or generated bindings.

## Inline Go with `@golang`

```goop
@golang {
  func greet(name string) string {
    return "Hello, " + name + "!"
  }
}
val greet : string -> string

let main () : unit =
  print_line (greet "Goop")
```

The `@golang { ... }` block is embedded Go. The `val` line declares the Goop-visible signature.

## Import Goop modules

```goop
import goop . "std.io"    (* dot import: PrintLine *)
import io goop "std.io"   (* qualified: io.PrintLine *)
```

See [modules guide](../design/05-modules-and-packages.md).

## Gradual migration

`.goop` and `.go` files coexist in one module. Migrate function by function; `go build` compiles the generated Go alongside hand-written Go.

Full example: [`extern_demo.goop`](../examples/extern_demo.goop).

## Next

[5. Concurrency →](05-concurrency.md)
