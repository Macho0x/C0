# C0 Modules and Packages

## Module declaration

Every C0 source file begins with a module declaration:

```c0
module MyModule
```

A module corresponds to a Go package. The module name determines:

- The emitted Go package name.
- The default file path within the project.
- The qualifier for imports from other C0 modules.

## Module naming

- Module names are `PascalCase` in C0.
- Lowered Go package names are `lowercase`.
- Multi-word modules use `UpperCamelCase` in C0 and `lowercase` in Go (e.g., `OrderBook` → `orderbook`).

## Opening modules

`open` brings a module's exported names into the current scope:

```c0
module Main

open Std.IO
open Math

let main () =
  Console.print_line (Float.to_string Math.pi)
```

`open` does not import foreign Go packages directly. It references other C0 modules. To bind a Go package, use `extern`.

## Import paths

The compiler maps C0 module names to import paths via the project configuration (`c0.toml`) or a default convention:

- `Std.IO` → `c0.dev/std/io`
- `MyProject.OrderBook` → `github.com/user/project/orderbook`

## Visibility

All top-level declarations are exported by default. Future versions may add an explicit `private` annotation.

```c0
let helper x = x + 1        (* exported: lowered to Helper *)
let main () = ...           (* exported: lowered to Main *)
```

## Hierarchical modules

C0 supports dotted module paths:

```c0
module Trading.OrderBook
```

This maps to the directory `trading/orderbook` and Go package `orderbook`.

## External Go packages

Use `extern` to declare bindings to Go packages:

```c0
extern "go" "fmt" {
  val print : string -> unit
  val printf : string -> 'a list -> unit
}
```

In v1, `extern` declarations are trusted: the C0 compiler does not verify them against the Go package. Incorrect signatures will produce errors at Go compile time.

## Compilation unit

A compilation unit is a directory containing `.c0` files that share the same module name. The compiler produces one Go package per module.

## Prelude

The compiler provides a small set of built-in bindings available to every C0 program without an explicit `open` statement. These bindings are in a synthetic "prelude" module that is implicitly opened in every file.

Prelude bindings are shadowable: a user-defined binding with the same name overrides the prelude version in that scope.

The following names are automatically in scope:

| Binding | C0 type | Go lowering |
|---|---|---|
| `print_line` | `string -> unit` | `fmt.Println` |
| `print` | `string -> unit` | `fmt.Print` |
| `int_to_string` | `int -> string` | `strconv.Itoa` |
| `float_to_string` | `float -> string` | `strconv.FormatFloat` (via `fmt.Sprintf`) |
| `string_concat` | `string -> string -> string` | `+` operator |
| `list_length` | `'a list -> int` | `len` (built-in) |
| `list_append` | `'a list -> 'a list -> 'a list` | `append` (built-in) |
| `panic_message` | `string -> 'a` | `panic` (built-in) |

The legacy name `Console.print_line` is also recognised for backward compatibility and maps to `fmt.Println`.
