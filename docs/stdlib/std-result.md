# std.result

**Source:** `std/result/result.goop`  
**Import:** `import goop "std.result"`

Thin helpers over the builtin `result` type. `Ok` and `Error` are language builtins.

## Exports

| Name | Type | Description |
|---|---|---|
| `isOk` | `('ok, 'err) result -> bool` | `true` if `Ok _` |
| `isError` | `('ok, 'err) result -> bool` | `true` if `Error _` |

## Example

```goop
import goop . "std.result"

let succeeded (r: (int, string) result) : bool =
  isOk r
```

For control flow, use `match` on `result` — see [tutorial chapter 3](../tutorial/03-errors-and-effects.md).
