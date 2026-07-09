# 1. Getting started

Goop compiles to Go. You write `.goop` files, type-check them, and emit readable Go source.

## Your first program

```goop
module main

let main () =
  print_line "Hello, Goop!"
```

`print_line` is a **prelude** binding — available in every file without an import. It lowers to `fmt.Println`.

## Build the compiler

```bash
cd src
go build -o ../goop ./cmd/goop
```

## Check a file

```bash
./goop check docs/examples/hello.goop
# OK: docs/examples/hello.goop parsed and type-checked successfully
```

`goop check` runs parsing, type inference, effect checking, and safety passes (exhaustiveness, linear analysis, nil channels, refinements).

## Project layout

```
my-project/
  goop.toml          # feature flags, std module mappings
  main.goop          # module main
  lib/
    utils.goop       # module Utils
```

Every file starts with `module Name`. The module name should match the intended Go package name.

## Functions and types

```goop
let double (x: int) : int = x + x

type color = Red | Green | Blue

let name (c: color) : string =
  match c with
  | Red -> "red"
  | Green -> "green"
  | Blue -> "blue"
```

Types are inferred when omitted. `match` must be **exhaustive** — the compiler rejects missing variants (see [chapter 6](06-safety-checks.md)).

## Run tests

```bash
./goop test tests/
```

## Next

[2. Types and patterns →](02-types-and-patterns.md)
