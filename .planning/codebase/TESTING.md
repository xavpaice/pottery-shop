# TESTING.md — Test Structure and Practices

## Framework
Go standard `testing` package

## Test File Organization
- Co-located with source: `product_test.go` alongside `product.go`
- 5 test files total covering models and middleware

## Test Naming Pattern
```
Test[FunctionName]_[Scenario]
```
Examples:
- `TestCreateAndGetByID`
- `TestBasicAuth_ValidCredentials`

## Test Helpers
- Helpers marked with `t.Helper()` for accurate failure reporting
- Setup helpers:
  - `setupTestStore()` — creates in-memory SQLite store
  - `createSampleProduct()` — seeds a test product
  - `setupTestEnv()` — configures environment for middleware tests

## Database Testing
- In-memory SQLite via `:memory:` connection string
- No external database required for unit tests
- Store interface allows swapping implementations

## HTTP Testing
- `httptest.NewRequest()` for crafting test HTTP requests
- `httptest.NewRecorder()` for capturing responses
- Middleware tested in isolation with mock next handlers

## Running Tests
```bash
make test            # Run all tests
make test-verbose    # Run with -v flag
make test-coverage   # Run with coverage report
```

## Coverage Areas
- `internal/models/` — data store operations (CRUD)
- `internal/middleware/` — auth middleware validation

## Integration Tests
- Integration test present (added in commit `8883512`)
- Likely tests full HTTP stack or database integration
