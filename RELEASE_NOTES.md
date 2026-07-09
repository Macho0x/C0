# Goop 0.7.1

## Parser & LSP
- **`let _`**: discard bindings are now accepted (e.g. `let _ = go (fun () -> ...)`)
- **LSP null responses**: hover, definition, and completion no longer emit malformed JSON-RPC when there is nothing to show
- **LSP I/O**: `readLSPMessage` checks `scanner.Err()` after reading headers

## Cleanup
- Removed unused parser and CLI helpers (`parseExternDecl`, `localGoopPathForImport`, `writeOpenDependencies`)
- Tidied `go.mod` (`golang.org/x/tools` is now a direct dependency)

## Tests
- `linear_go_handoff_test.goop` uses idiomatic `let _` syntax
- Full `go test ./...` and 32 `goop test` integration tests passing
