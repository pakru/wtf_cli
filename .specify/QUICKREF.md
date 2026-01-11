# WTF CLI Constitution - Quick Reference

## Pre-Commit Checklist ✅

Before committing any code, verify:

```bash
# 1. Format code
make fmt

# 2. Run vet
make vet

# 3. Run all tests
make test

# 4. Check coverage (target: >80%)
make test-coverage

# 5. Run linter (if available)
make lint

# 6. Test in dry-run mode
WTF_DRY_RUN=true ./wtf_cli
```

## Code Quality Standards

### Function Guidelines
- **Max 50 lines** per function (preferred)
- **Cyclomatic complexity ≤ 10**
- **Single responsibility** - one function, one purpose
- **Godoc comments** for all exported items

### Error Handling
```go
// ❌ BAD: Silent failure
result, _ := doSomething()

// ✅ GOOD: Explicit handling
result, err := doSomething()
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}
```

### Constants
```go
// ❌ BAD: Magic numbers
if timeout > 30 { ... }

// ✅ GOOD: Named constants
const DefaultAPITimeout = 30 * time.Second
if timeout > DefaultAPITimeout { ... }
```

## TDD Workflow

### Red-Green-Refactor Cycle
1. **Red**: Write failing test
   ```bash
   go test ./... # Should fail
   ```

2. **Green**: Implement minimal code
   ```bash
   go test ./... # Should pass
   ```

3. **Refactor**: Improve while keeping tests green
   ```bash
   go test ./... # Should still pass
   ```

### Test Structure
```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "test", "result", false},
        {"empty input", "", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Feature(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Performance Targets

| Metric | Target | How to Measure |
|--------|--------|----------------|
| Time-to-first-output | <200ms | `time ./wtf_cli` (excluding API) |
| Shell integration overhead | <10ms | `time ls` with/without integration |
| Configuration loading | <50ms | Benchmark test |
| System info gathering | <100ms | Benchmark test |
| Binary size | <20MB | `ls -lh wtf_cli` |
| Memory usage | <50MB | `ps aux` or profiling |

### Running Benchmarks
```bash
# Run all benchmarks
go test -bench=. ./...

# Run specific benchmark
go test -bench=BenchmarkConfigLoad ./config

# With memory profiling
go test -bench=. -benchmem ./...
```

## UX Standards

### Output Format
```
🧪 [Mode] - [Status]
═══════════════════════
[Context Information]
Original command: [command]
Exit code: [code]
OS: [os] [version]

💡 [Suggestions Section]:
   • [Suggestion 1]
   • [Suggestion 2]
```

### Error Messages
```go
// ❌ BAD: Vague error
return errors.New("failed")

// ✅ GOOD: Actionable error
return fmt.Errorf("failed to load config from %s: %w\nTry: wtf --init to create default config", path, err)
```

### Environment Variables
All environment variables MUST use `WTF_*` prefix:
- `WTF_DEBUG` - Enable debug mode
- `WTF_DRY_RUN` - Enable dry-run mode
- `WTF_API_KEY` - Override API key
- `WTF_LOG_LEVEL` - Set log level
- `WTF_LAST_*` - Shell integration variables

## Common Patterns

### Structured Logging
```go
import "log/slog"

// Info level
logger.Info("loading configuration", "path", configPath)

// Debug level
logger.Debug("API request", "model", model, "tokens", maxTokens)

// Error with context
logger.Error("API call failed", "error", err, "status", statusCode)
```

### Mock External Dependencies
```go
// Define interface
type APIClient interface {
    SendRequest(ctx context.Context, req Request) (Response, error)
}

// Use interface in code
func ProcessCommand(client APIClient, cmd string) error {
    // ...
}

// Mock in tests
type mockClient struct {
    response Response
    err      error
}

func (m *mockClient) SendRequest(ctx context.Context, req Request) (Response, error) {
    return m.response, m.err
}
```

## Integration Testing

### Shell Integration E2E
```bash
#!/bin/bash
# Test shell integration
source ~/.wtf/integration.sh

# Run command
ls /nonexistent 2>&1

# Verify JSON created
test -f ~/.wtf/last_command.json || exit 1

# Verify content
grep '"command"' ~/.wtf/last_command.json || exit 1
```

### API Contract Testing
```go
func TestOpenRouterAPI(t *testing.T) {
    // Mock server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify request structure
        // Return mock response
    }))
    defer server.Close()
    
    // Test with mock
    client := NewClient(server.URL, "test-key")
    // ...
}
```

## Violation Documentation

If you must violate a principle, document in plan.md:

```markdown
## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Function >50 lines | Complex parsing logic | Breaking into smaller functions reduces readability |
| New dependency | Required for feature X | No standard library equivalent available |
```

## Quick Commands

```bash
# Development workflow
make dev              # fmt + vet + test + build

# Testing
make test             # Run all tests
make test-coverage    # Generate coverage
make coverage-html    # View coverage in browser

# Installation
./scripts/install.sh  # Full install with shell integration

# Dry-run testing
WTF_DRY_RUN=true ./wtf_cli

# Debug mode
WTF_DEBUG=true WTF_LOG_LEVEL=debug ./wtf_cli
```

---

**Remember**: When in doubt, refer to `.specify/memory/constitution.md` for full details.
