# Goop standard library reference

Goop has three API layers:

| Layer | Import needed? | Location |
|---|---|---|
| **Language builtins** | No | `option`, `result`, `'a list`, `chan`, etc. |
| **Prelude** | No | Always in scope; defined in `src/internal/prelude/prelude.go` |
| **`std.*` modules** | Yes — `import goop "std.io"` | `std/` directory |

For production code, **Go’s standard library** is the main dependency — use `import golang "net/http" { ... }`. The shipped `std.*` modules are a small Goop-native supplement.

## Prelude

[Prelude reference](prelude.md) — `print_line`, `Chan.*`, `OwnedChan.*`, string helpers, assertions.

## Builtins

[Language builtins](builtins.md) — primitive types, `list`, `option`, `result`.

## std.* modules

| Module | Import path | Reference |
|---|---|---|
| `std.io` | `import goop "std.io"` or `import goop . "std.io"` | [std.io](std-io.md) |
| `std.list` | `import goop "std.list"` | [std.list](std-list.md) |
| `std.option` | `import goop "std.option"` | [std.option](std-option.md) |
| `std.result` | `import goop "std.result"` | [std.result](std-result.md) |

## Import forms

```goop
import goop "std.io"           (* qualified: must use module exports by name *)
import goop . "std.io"         (* dot: PrintLine in scope *)
import io goop "std.io"        (* alias: io.PrintLine *)
```

Resolution is configured in `goop.toml` `[mappings]` and defaults in the compiler. See [modules guide](../design/05-modules-and-packages.md).

## Naming conventions

| Layer | Convention | Example |
|---|---|---|
| Prelude | `snake_case` | `print_line`, `int_to_string` |
| `std.*` | `PascalCase` for exports | `PrintLine`, `Map` |
| Constructors | `PascalCase` | `Some`, `Ok`, `OrderId` |

## Maintenance

This reference is hand-written from compiler sources (`prelude.go`, `std/*/*.goop`). When adding prelude bindings or `std.*` exports, update the matching page here.

Automated `goop doc` generation is not implemented yet ([TODO](../../TODO.md)).
