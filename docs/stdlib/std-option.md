# std.option

**Source:** `std/option/option.goop`  
**Import:** `import goop "std.option"`

Thin helpers over the builtin `option` type. `None` and `Some` are language builtins — no import required.

## Exports

| Name | Type | Description |
|---|---|---|
| `isSome` | `'a option -> bool` | `true` if `Some _` |
| `isNone` | `'a option -> bool` | `true` if `None` |

## Example

```goop
import goop . "std.option"

let describe (x: int option) : string =
  if isSome x then "present" else "absent"
```

Prefer `match` when you need the payload — `isSome` / `isNone` are for boolean checks only.
