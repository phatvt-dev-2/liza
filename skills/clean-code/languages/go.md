# Go — Clean Code Language Profile

## Tool Map

| Variable | Command | Notes |
|----------|---------|-------|
| `$TEST_CMD` | `go test ./...` | |
| `$COVERAGE_CMD` | `go test -coverprofile=coverage.out ./... && go tool gocover-cobertura < coverage.out > coverage.xml` | Pinned in `go.mod` via `tool` directive |
| `$COVERAGE_REPORT` | `coverage.xml` | Cobertura XML via `gocover-cobertura` |
| `$TYPE_CHECKER` | `go vet ./...` | Go is statically typed — `go vet` catches correctness issues beyond the compiler |
| `$DEAD_CODE_TOOL` | `deadcode ./...` | Requires `go install golang.org/x/tools/cmd/deadcode@latest`. Also: `staticcheck` with `U1000` check |
| `$IMPORT_CHECKER` | `goimports -l <files>` | Detects unused/missing imports and formatting |
| `$CYCLE_CHECKER` | `go build ./...` | Compiler rejects import cycles. For visualization: `godepgraph` |

If `gocover-cobertura` is unavailable (non-project context): fall back to `go test -coverprofile=coverage.out` and check coverage manually with `go tool cover -func=coverage.out`. Warn that `diff-cover` integration is unavailable.

## Performance Patterns

| Transformation | Potential perf impact |
|---------------|---------------------|
| Pointer receiver → value receiver | Copies struct per call; significant for large structs in hot paths |
| `interface{}` → concrete type | Eliminates boxing/unboxing; enables compiler optimizations |
| String concat in loop → `strings.Builder` | Avoids O(n^2) allocation; critical in loops |
| `map` for small lookups (≤10 keys) | Slice/switch is faster — map has hash overhead |
| Channel → mutex for simple sync | Channels have goroutine scheduling overhead; mutex is lighter for protect-and-release |
| `fmt.Sprintf` in hot path | Reflection-based; use `strconv` or `strings.Builder` for known types |

## Idiom Patterns

| Signal | Transformation |
|--------|----------------|
| `panic` for expected errors | Return `error` — panic is for truly unrecoverable situations |
| Bare `error` return without context | Wrap with `fmt.Errorf("context: %w", err)` |
| `interface{}` / `any` where type is known | Use concrete type or type parameter (Go ≥1.18) |
| Manual cleanup (`Close()` after `Open()`) | Use `defer` immediately after acquisition |
| Named return values not used for documentation | Remove named returns unless used for defer clarity or godoc |
| Exported names without doc comments | Add doc comment (`// FuncName does...`) — required by `revive`/`staticcheck` |
| `init()` functions with side effects | Move to explicit initialization; `init()` complicates testing and ordering |
| Empty struct for set (`map[K]bool`) | Use `map[K]struct{}` for zero-allocation set membership |
| Long function with multiple concerns | Split; Go idiom favors small functions with single return path |
| Raw `map[string]interface{}` for structured data | Define a struct |
| Error variable naming (`err2`, `err3`) | Shadow `err` with `:=` in each block, or extract to function |
| `sync.Mutex` embedded without comment | Group mutex with the fields it protects; comment the invariant |
| Goroutine leak (unbounded `go func()`) | Ensure goroutines have exit conditions; use `errgroup` or context cancellation |
| String comparison for enums | Define `type X int` with `iota` constants |
| Large interface as function parameter | Accept small interfaces — prefer `io.Reader` over `*os.File`; "accept interfaces, return structs" |
| Interface defined next to implementation | Define interfaces at the call site (consumer), not the provider — lets consumers declare only what they need |
| `context.Context` stored in struct field | Pass `ctx` as first parameter; storing it breaks cancellation propagation |
| Type assertion on error (`err.(*MyError)`) | Use `errors.As` — works through wrapping chains (Go ≥1.13) |
| `err == ErrNotFound` equality check | Use `errors.Is` — works through wrapping chains (Go ≥1.13) |
| Returned error not checked | Handle or explicitly ignore with `_ =` + comment explaining why; `errcheck` linter catches this |
| Struct zero value is unusable (requires `NewX()`) | Design structs so zero value is valid and useful — reduces nil-check burden on callers |
| Constructor with >3 config params | Use functional options pattern (`WithTimeout(d)`, `WithLogger(l)`) or config struct |
| Package named `util`, `common`, `helpers` | Name packages by what they *provide*, not what they *contain* — `http`, `auth`, `store` |
| Repetitive test cases with same structure | Table-driven tests with `t.Run(name, ...)` subtests |
| Test helper without `t.Helper()` call | Add `t.Helper()` so failures report the caller's line, not the helper's |
