# Goop 1.1.1

Tests and examples overhaul: prune stale demos, strengthen e2e coverage, fix `select` channel lowering.

## Highlights

- **Examples:** Removed overlapping demos (`chan_async`, `channel_race`, `linear`, `match_patterns`, `simple_hl_bot`, `trading_position`); fixed `concurrency.goop` (go + send + recv); added `modules.goop` and `exceptions.goop`
- **Tests:** Dropped non-CI Binance demo and duplicate refinement test; renamed misleading `async`/`guards`/`newtype` test files; real asserts for FCM, synthetic TA, list/array helpers
- **Coverage:** `select_test`, `chan_close_test`, `effect_multi_test`, `exception_payload_test`; Go unit tests for `perform`-in-`go` and LINEAR008 race fixtures
- **Compiler:** `select` lowers through `C0Chan.ch` with element type assertions; `checkPerformInGo` walks `ParenExpr`

## Verification

- `go test ./...` (from `src/`)
- `goop test tests/` — 68 passed
- `goop check` on all `docs/examples/*.goop`
