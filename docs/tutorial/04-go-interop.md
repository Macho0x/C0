# 4. Go / C interop

Goop’s primary standard library is **Go itself**. Use `import go` for packages and `@[go]` for inline Go. For C, use `@[c]` (cgo-shaped).

## Import Go packages

```goop
module main

import (
  go "strings" {
    val ToUpper : string -> string
  }
  go "fmt"
)
```

- **Signature block** — declare types for functions you call from Goop.
- **Import-only** — `go "fmt"` with no block; pair with `@[go]` or generated bindings.

## Implement Go interfaces

Import an interface as an opaque Go type, then use `implements` to generate
its pointer-receiver method set from native Goop methods:

```goop
import go "fmt" {
  type Stringer
}

type point = { x : int; y : int }

implements Stringer for point with
  let String (p : point) : string =
    int_to_string p.x ^ "," ^ int_to_string p.y
end
```

This emits a Go assertion that `*point` satisfies `fmt.Stringer`; `@[go]` is
not needed for method bodies expressible in Goop. For complete examples, see
[`go_implements_stringer.goop`](../examples/go_implements_stringer.goop) and
[`go_implements_slog_handler.goop`](../examples/go_implements_slog_handler.goop),
which implements a native `slog.Handler`.

## Inline Go with `@[go]`

```goop
@[go] {
  func greet(name string) string {
    return "Hello, " + name + "!"
  }
}
val greet : string -> string

let main () : unit =
  print_line (greet "Goop")
```

The `@[go] { ... }` block is embedded Go. The `val` line declares the Goop-visible signature.

## Inline C with `@[c]`

```goop
@[c] {
  #include <string.h>
  int add(int a, int b) { return a + b; }
}
val add : int -> int -> int
```

Bodies become a cgo preamble (`import "C"`). Primitive `val` types are auto-wrapped; richer FFI uses `@[go]` calling `C.*`. See [15-lang-embeds.md](../design/15-lang-embeds.md).

## Import Goop modules

```goop
import goop . "std.io"    (* dot import: PrintLine *)
import io goop "std.io"   (* qualified: io.PrintLine *)
```

See [modules guide](../design/05-modules-and-packages.md).

## Gradual migration

`.goop` and `.go` files coexist in one module. Migrate function by function; `go build` compiles the generated Go alongside hand-written Go.

Full examples: [`extern_demo.goop`](../examples/extern_demo.goop), [`cgo_demo.goop`](../examples/cgo_demo.goop).

## Next

[5. Concurrency →](05-concurrency.md)
