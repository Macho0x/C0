# Goop Package Manager (v0.3.0)

Goop v0.3.0 adds a minimal package manager aligned with Go module paths.

## `goop get`

Fetch a remote Goop module and pin it in the project:

```bash
goop get github.com/acme/lib
goop get github.com/acme/lib@v1.2.3
```

This:

1. Clones the repository into `$GOOP_HOME/pkg/mod` (default `~/.cache/goop/pkg/mod`).
2. Appends an entry to `goop.lock`.
3. Adds `[dependencies]` in `goop.toml`.

## `goop.lock`

Pinned modules at the project root:

```toml
[[module]]
path = "github.com/acme/lib"
version = "v1.2.3"
source = "github.com/acme/lib"
```

The compiler prefers lock pins over floating `goop.toml` mappings.

## `goop.toml` dependencies

```toml
module_root = "github.com/you/yourapp"

[mappings]
"std.io" = "github.com/Macho0x/Goop/std/io"

[dependencies]
"github.com/acme/lib" = "v1.2.3"
```

## Module entry convention

Remote Goop modules are discovered at:

`<module>/<last-segment>/<last-segment>.goop`

Example: `github.com/Macho0x/Goop/std/io/io.goop`.

## `goop resolve`

Print import resolution and the transitive `import goop` graph:

```bash
goop resolve main.goop
```
