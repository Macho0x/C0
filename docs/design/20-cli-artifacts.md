# CLI artifacts: compile, build, test

Goop lowers to Go, but **developers work with `.goop` only**. Generated `.go`
and optional source maps live under `$GOOP_HOME` (default `~/.cache/goop`).

## Locations

| Path | Purpose |
|------|---------|
| `$GOOP_HOME/pkg/mod` | Cloned modules from `goop get` |
| `$GOOP_HOME/build/compile-*` | `goop compile` output |
| `$GOOP_HOME/build/build-*` | `goop build` sandbox (entry + deps + `go.mod`) |
| `$GOOP_HOME/build/test-*` | `goop test` sandbox (removed after each test) |

## Commands

### `goop check <file.goop>`

Type-check and safety only. Writes nothing.

### `goop compile <file.goop>`

Emits Go into `$GOOP_HOME/build/compile-*` and prints the path.
Does not run `go build`.

### `goop build <file.goop>`

1. Type-check + safety on the entry file
2. Compile transitive `import goop` deps into the build sandbox
3. Run `go build` in the sandbox
4. For programs with `func main`, write `./goop-out` in the current working directory

Does **not** leave `.go`, `.map.json`, or `go.mod` in the source tree.

### `goop test [dir]`

Discovers `*_test.goop`, builds each in an ephemeral sandbox (same dep wiring
as `goop build`), runs the binary, then deletes the sandbox.

## Flags

| Flag | Default | Effect |
|------|---------|--------|
| (none) | cache | Write under `$GOOP_HOME/build` |
| `--in-tree` | off | Write `.go` beside the `.goop` source |
| `--emit-map` | off | Write `.map.json` next to the generated `.go` |
| `--no-source-map` | — | Accepted; maps stay off (compat with older scripts) |

## Mixed Go + Goop

Use `--in-tree` when a package already contains hand-written `.go` files that
must compile together with the generated output.
