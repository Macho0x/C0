# Goop language tutorial

A step-by-step introduction to Goop. Each chapter links to runnable examples checked by CI (`goop check`).

| Chapter | Topic | Example |
|---|---|---|
| [1. Getting started](01-getting-started.md) | Build, check, first program | [`hello.goop`](../examples/hello.goop) |
| [2. Types and patterns](02-types-and-patterns.md) | ADTs, `match`, records | [`shapes.goop`](../examples/shapes.goop) |
| [3. Errors and effects](03-errors-and-effects.md) | `result`, `?`, `with { }` | [`result.goop`](../examples/result.goop), [`effects.goop`](../examples/effects.goop) |
| [4. Go interop](04-go-interop.md) | `import golang`, `@golang` | [`extern_demo.goop`](../examples/extern_demo.goop) |
| [5. Concurrency](05-concurrency.md) | `go`, `chan`, race checks | [`concurrency.goop`](../examples/concurrency.goop), [`race_detection.goop`](../examples/race_detection.goop) |
| [6. Safety checks](06-safety-checks.md) | Exhaustiveness, newtypes, nil channels | [`newtype_trading.goop`](../examples/newtype_trading.goop), [`trading_order.goop`](../examples/trading_order.goop) |

## Prerequisites

```bash
cd src && go build -o ../goop ./cmd/goop
../goop check ../docs/examples/hello.goop
```

## Editor setup

- **VS Code**: install [`editors/vscode`](../../editors/vscode) — syntax highlighting, `.goop` file icon, LSP
- **Zed**: `cd editors/zed && make install`
- **GitHub**: interim F# highlighting via [`.gitattributes`](../../.gitattributes); full Goop highlighting after [Linguist submission](github-linguist/README.md)

## Further reading

- [Syntax reference](../design/03-syntax.md)
- [Type system](../design/02-type-system.md)
- [Standard library reference](../stdlib/README.md)
- [All examples](../examples/)
