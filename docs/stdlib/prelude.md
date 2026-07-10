# Prelude reference

The prelude is injected by the type checker before user declarations. Bindings are **shadowable** — a local `let print_line = ...` hides the prelude version.

**Source:** `src/internal/prelude/prelude.go`

## I/O

| Name | Type | Go lowering |
|---|---|---|
| `print_line` | `string -> unit` | `fmt.Println` |
| `print` | `string -> unit` | `fmt.Print` |
| `Console.print_line` | `string -> unit` | `fmt.Println` |

## Strings and numbers

| Name | Type | Go lowering |
|---|---|---|
| `int_to_string` | `int -> string` | `strconv.Itoa` |
| `float_to_string` | `float -> string` | `fmt.Sprintf` |
| `string_concat` | `string -> string -> string` | `+` operator |

In practice, use the `^` operator for string concatenation.

## Lists

| Name | Type | Go lowering |
|---|---|---|
| `list_length` | `'a list -> int` | `len` |
| `list_append` | `'a list -> 'a list -> 'a list` | `append` |

List syntax (`[]`, `::`, `[a; b]`) is built into the language — see [builtins](builtins.md).

## Arrays

| Name | Type | Go lowering |
|---|---|---|
| `Array.make` | `int -> 'a -> 'a array` | `make([]T, n)` + fill loop |
| `Array.length` | `'a array -> int` | `len` |

Index read `arr.(i)` and write `arr.(i) <- v` are language syntax — see [std.array](std-array.md).

## Refs and abort

| Name | Type | Go lowering |
|---|---|---|
| `ref` | `'a -> 'a ref` | pointer allocation |
| `failwith` | `string -> 'a` | `panic(...)` |
| `assert` | `bool -> unit` | `if !b { panic(...) }` |
| `assert_equal` | `'a -> 'a -> unit` | equality panic |

`!r` and `r := e` are language syntax. There is no `panic_message` — use `failwith`.

## Channels (`chan`)

| Name | Type |
|---|---|
| `Chan.make` | `unit -> 'a chan` |
| `Chan.send` | `'a chan -> 'a -> unit` |
| `Chan.recv` | `'a chan -> 'a` |
| `Chan.close` | `'a chan -> unit` |

`Chan.close` is runtime-safe (closed-flag wrapper). Nil-channel use before init is caught by **NIL001**.

## Linear channels (`owned_chan`)

| Name | Type |
|---|---|
| `OwnedChan.make` | `unit -> 'a owned_chan` |
| `OwnedChan.send` | `'a owned_chan -> 'a -> unit` |
| `OwnedChan.recv` | `'a owned_chan -> 'a` |
| `OwnedChan.close` | `'a owned_chan -> unit` |

Linear discharge checking ensures channels are closed exactly once. See [`owned_chan.goop`](../examples/owned_chan.goop).

## HTTP / JSON helpers

Trading-oriented helpers lowering to inline Go:

| Name | Type |
|---|---|
| `http_get_string` | `string -> string` |
| `json_extract_floats` | `string -> int -> float list` |
| `json_extract_strings` | `string -> int -> string list` |

Used by trading demos that call live or synthetic market data helpers (see [`allmids_bot.goop`](../examples/allmids_bot.goop)).
