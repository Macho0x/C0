## Source style

C0 uses **offside-rule** syntax (like F# and Python): indentation defines block structure. The language is case-sensitive.

Comments:

```c0
(* This is a block comment. *)
(* Nested comments (* are supported *). *)
```

Line comments are not provided by default; block comments are sufficient and align with OCaml tradition.

## Identifiers and keywords

- Identifiers start with a letter or underscore and continue with letters, digits, underscores, or apostrophes.
- Type variables are written with a leading apostrophe: `'a`, `'key`.
- Keywords include: `let`, `type`, `match`, `with`, `if`, `then`, `else`, `fun`, `module`, `open`, `import`, `extern`, `mutable`, `rec`, `and`, `in`, `as`, `when`, `requires`, `returns`, `true`, `false`, `unit`.

## Modules

A file begins with a module declaration:

```c0
module MyModule

open Std.List
open Other.Module

let x = 1
```

Modules map to Go packages. `open` brings names into scope without qualifying them; `import` references a Go package.

## Value declarations

```c0
let pi = 3.14159

let square (x: float) : float = x *. x

let rec factorial (n: int) : int =
  if n <= 1 then 1
  else n * factorial (n - 1)
```

Mutability is explicit:

```c0
let mutable counter = 0
counter <- counter + 1
```

## Functions

Functions are curried by default:

```c0
let add (x: int) (y: int) : int = x + y
let addFive = add 5
```

Anonymous functions:

```c0
let f = fun x -> x + 1
let doubled = List.map (fun x -> x * 2) numbers
```

## Type declarations

Records:

```c0
type point = { x: float; y: float }
```

ADTs:

```c0
type option 'a = None | Some of 'a
```

Type aliases:

```c0
type user_id = string
```

Linear resource types (opt-in modal linearity):

```c0
type handle : 1
```

Types without `: 1` are unrestricted (`ω`). Linear types must be discharged (used/handed-off) on every control-flow path. See `docs/examples/linear.c0`.

## Effect row annotations

Effect rows appear after a function return type with `with`:

```c0
(* Explicitly pure *)
let double (x: int) : int with {} = x * 2

(* Has IO effect *)
let readFile (path: string) : string with { io } = ...

(* Multiple effects *)
let complex () : unit with { io; log } = ...

(* Row-polymorphic: at least state, plus any others *)
let withState (f: unit -> 'a with { state | e }) : 'a with { e } = ...
```

Effect rows are compile-time only and erased in Go output. See `docs/design/02-type-system.md` and `docs/examples/effects.c0`.

## Refinement `where` clauses

`where` is a postfix type modifier for runtime contract assertions:

```c0
(* `it` refers to the parameter value *)
let safeDiv (a: int) (b: int where b <> 0) : int = a / b

(* `result` refers to the return value *)
let clamp (x: int) (lo: int) (hi: int where hi >= lo) : int where result >= lo && result <= hi = ...
```

Refinements lower to runtime `panic` guards. No SMT solver is involved. See `docs/design/02-type-system.md` and `docs/examples/contracts.c0`.

## Pattern matching

```c0
match expr with
| Pattern1 -> expr1
| Pattern2 when guard -> expr2
| _ -> default_expr
```

Patterns include:

- Wildcards: `_`
- Literals: `42`, `"hello"`, `true`
- Constructors: `Some x`, `Circle { radius }`
- Tuples: `(x, y)`
- Lists: `[]`, `x :: xs`, `[a; b; c]`
- Records: `{ x; y }`, `{ name; age }`
- Aliases: `p as point`
- Guards: `when x > 0`

## Match macros

C0 provides three syntactic shortcuts that desugar to `match`:

### `is`

```c0
if status is Passed then ...
```

### `as`

```c0
let name = getUser(id) as Some {name, ..} -> name else "Anonymous"
```

### `guard`

```c0
let processUser (id: int) : result<user, string> =
  guard Some user = findUser id else Error "not found"
  guard Ok validated = validate user else Error "validation failed"
  Ok (transform validated)
```

## Error propagation

The `?` operator propagates `result` errors:

```c0
let readConfig (path: string) : result<config, error> =
  let bytes = File.readAllBytes path ?
  let text = Encoding.utf8.getString bytes ?
  let config = Json.parse<config> text ?
  Ok config
```

Three forms are supported:

```c0
let x = f() ?                           (* bare *)
let x = f() ? "context message"         (* wrap message *)
let x = f() ? fun e -> wrapError e      (* transform error *)
```

## Pipelines

C0 uses F#-style data-first pipelines:

```c0
let result =
  data
  |> List.filter (fun x -> x > 0)
  |> List.map (fun x -> x * 2)
  |> List.fold (fun acc x -> acc + x) 0
```

## Operator precedence

Arithmetic and comparison operators follow ML precedence. Logical operators: `&&` (and), `||` (or), `not`. Function application binds tighter than operators.

## Extern and Go interop

Call into Go directly:

```c0
extern "go" "github.com/example/lib" {
  val loadConfig : string -> result<config, error>
}
```

## Computation expressions

C0 supports F#-style computation expressions for monadic programming. Two builders are provided: `result` and `async`.

### `result { ... }`

The `result` builder desugars `let!` bindings into nested `match` expressions that propagate errors:

```c0
let safeDiv (x: float) (y: float) : (float, string) result =
  if y = 0.0 then Error "division by zero"
  else Ok (x *. y)

result {
  let! a = safeDiv 10.0 2.0
  let! b = safeDiv a 3.0
  return a *. b
}
```

Desugars to:
```c0
match safeDiv 10.0 2.0 with
| Ok a ->
    match safeDiv a 3.0 with
    | Ok b -> Ok (a *. b)
    | Error e -> Error e
| Error e -> Error e
```

Operations inside `result { }`:
- `let! pattern = expr` — bind a value from a result
- `do! expr` — execute for effects (like `let! _ = expr`)
- `return expr` — wrap in `Ok`
- `return! expr` — return a result directly
- `let pattern = expr` — regular let binding
- Final expression — the body

### `region { ... }`

The `region` builder provides scoped resource management with guaranteed cleanup:

```c0
type handle : 1

let Close (h: handle) : unit = ...

let useResource (h: handle) : unit =
  region {
    let! x = h                     (* acquires a linear resource *)
    do! useHandle x                (* uses the resource *)
    return ()                      (* returns the result *)
  }
```

Operations inside `region { }`:
- `let! pattern = expr` — acquires a linear resource (emits `defer Close(varName)` in Go output). The variable is auto-discharged at region exit.
- `let pattern = expr` — binds a non-linear value.
- `do! expr` — performs an effect.
- `return expr` — produces the final result.

The linear discharge checker auto-discharges region-bound variables at scope exit. This replaces the legacy `using` block with compile-time-guaranteed cleanup.

See `docs/examples/region.c0`.
