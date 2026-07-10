# Goop 1.0.1

Docs and README cleanup after the 1.0.0 mega release.

## Highlights

- **README** first viewport shortened for first-time GitHub visitors; status set to shipped **1.0.1**
- **Example renames:** `branded_ids`, `match_patterns`, `linear_resource`, `chan_async`, `result_match` (unique bits from `using.goop` folded into `linear_resource`)
- **Effects:** real minimal `effect` / `perform` / handler demo in `docs/examples/effects.goop`; stronger e2e `effect_handler_test.goop`
- **Doc accuracy:** PARSE019/020 obsolete → PARSE-MIG016; banner on deferred-features analysis; STYLE prefers plain `match` on `result` (not `let*`); overview OCaml + Go pitch without F# peer

## Verification

- `go test ./...` (from `src/`)
- `goop test tests/` — 49 passed
- `goop check` on all `docs/examples/*.goop`
