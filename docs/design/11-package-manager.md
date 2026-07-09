# C0 Package Manager (v0.3.0)

C0 v0.3.0 adds a minimal package manager aligned with Go module paths.

## `c0 get`

Fetch a remote C0 module and pin it in the project:

```bash
c0 get github.com/acme/lib
c0 get github.com/acme/lib@v1.2.3
```

This:

1. Clones the repository into `$C0_HOME/pkg/mod` (default `~/.cache/c0/pkg/mod`).
2. Appends an entry to `c0.lock`.
3. Adds `[dependencies]` in `c0.toml`.

## `c0.lock`

Pinned modules at the project root:

```toml
[[module]]
path = "github.com/acme/lib"
version = "v1.2.3"
source = "github.com/acme/lib"
```

The compiler prefers lock pins over floating `c0.toml` mappings.

## `c0.toml` dependencies

```toml
module_root = "github.com/you/yourapp"

[mappings]
"std.io" = "github.com/Macho0x/C0/std/io"

[dependencies]
"github.com/acme/lib" = "v1.2.3"
```

## Module entry convention

Remote C0 modules are discovered at:

`<module>/<last-segment>/<last-segment>.c0`

Example: `github.com/Macho0x/C0/std/io/io.c0`.

## `c0 resolve`

Print import resolution and the transitive `import c0` graph:

```bash
c0 resolve main.c0
```
