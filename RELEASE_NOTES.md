# Goop 1.1.0

OCaml parity mega-ship: close remaining syntax/semantics gaps after 1.0.

## Highlights

- **Control:** `downto`, local/`let open`/`open!`, `exception` patterns, real memoizing `Lazy`
- **Modules:** inline sealing (`module M : S = …` — **no `.mli`**), `module type of`, `module rec`, signature constraints, functors, first-class modules, `include` re-exports
- **Types:** extensible variants, GADTs (result types), polyvar rows, labelled/optional args
- **Objects:** inherit/virtual/initializer/constraint/class type + `#method`
- **Effects:** shallow CPS resume with `continue`/`discontinue`
- **Attributes:** parse+strip only; `@golang { }` stays the only active extension

## Out of scope (unchanged)

- Deep effect handlers / stack capture
- PPX plugins / deriving
- Full OCaml Stdlib / Domains
- `.mli` interface files

## Verification

- `go test ./...` (from `src/`)
- `goop test tests/` — 66 passed
- `goop check` on all `docs/examples/*.goop`
