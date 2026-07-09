# Prelude reference

The prelude is injected by the type checker before user declarations. Bindings are **shadowable** — a local `let print_line = ...` hides the prelude version.

**Source:** `src/internal/prelude/prelude.go`

## I/O

| Name | Type | Effects | Go lowering |
|---|---|---|---|
| `print_line` | `string -> unit` | `io` | `fmt.Println` |
| `print` | `string -> unit` | `io` | `fmt.Print` |
| `Console.print_line` | `string -> unit` | `io` | `fmt.Println` |

Annotate callers with `with { io }` when effect inference is off or when you declare a pure signature explicitly.

## Strings and numbers

| Name | Type | Effects | Go lowering |
|---|---|---|---|
| `int_to_string` | `int -> string` | `{}` | `strconv.Itoa` |
| `float_to_string` | `float -> string` | `{}` | `fmt.Sprintf` |
| `string_concat` | `string -> string -> string` | `{}` | `+` operator |

In practice, use the `^` operator for string concatenation.

## Lists

| Name | Type | Effects | Go lowering |
|---|---|---|---|
| `list_length` | `'a list -> int` | `{}` | `len` |
| `list_append` | `'a list -> 'a list -> 'a list` | `{}` | `append` |

List syntax (`[]`, `::`, `[a; b]`) is built into the language — see [builtins](builtins.md).

## Panic and assertions

| Name | Type | Effects | Go lowering |
|---|---|---|---|
| `panic_message` | `string -> 'a` | `panic` | `panic(...)` |
| `assert` | `bool -> unit` | `panic` | `if !b { panic(...) }` |
| `assert_equal` | `'a -> 'a -> unit` | `panic` | equality panic |

## Channels (`chan`)

| Name | Type | Effects |
|---|---|---|
| `Chan.make` | `unit -> 'a chan` | `{}` |
| `Chan.send` | `'a chan -> 'a -> unit` | `async` |
| `Chan.recv` | `'a chan -> 'a` | `async` |
| `Chan.close` | `'a chan -> unit` | `async` |

`Chan.close` is runtime-safe (closed-flag wrapper). Nil-channel use before init is caught by **NIL001**.

## Linear channels (`owned_chan`)

| Name | Type | Effects |
|---|---|---|
| `OwnedChan.make` | `unit -> 'a owned_chan` | `{}` |
| `OwnedChan.send` | `'a owned_chan -> 'a -> unit` | `async` |
| `OwnedChan.recv` | `'a owned_chan -> 'a` | `async` |
| `OwnedChan.close` | `'a owned_chan -> unit` | `async` |

Linear discharge checking ensures channels are closed exactly once. See [`owned_chan.goop`](../examples/owned_chan.goop).

## HTTP / JSON helpers

Trading-oriented helpers lowering to inline Go:

| Name | Type | Effects |
|---|---|---|
| `http_get_string` | `string -> string` | `io` |
| `json_extract_floats` | `string -> int -> float list` | `{}` |
| `json_extract_strings` | `string -> int -> string list` | `{}` |

Used in [`simple_hl_bot.goop`](../examples/simple_hl_bot.goop) and related examples.
