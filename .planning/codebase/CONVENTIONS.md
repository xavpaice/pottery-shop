# CONVENTIONS.md — Code Style and Patterns

## Language
Go (Golang)

## File Naming
- Source files: lowercase with underscores (e.g., `product.go`, `basic_auth.go`)
- Test files: `_test.go` suffix co-located with source (e.g., `product_test.go`)

## Naming Conventions

### Exported (public) identifiers
- PascalCase for functions, types, structs, interfaces, constants
- Examples: `CreateProduct`, `GetByID`, `ProductStore`, `Product`

### Unexported (private) identifiers
- camelCase for variables, functions
- Examples: `setupTestStore`, `createSampleProduct`

### Database schema
- snake_case for column names: `product_id`, `is_sold`, `created_at`

### Go struct fields
- PascalCase mirroring DB columns: `ProductID`, `Title`, `IsSold`, `CreatedAt`

## Package Organization
```
cmd/server/          - Application entry point (main)
internal/handlers/   - HTTP handler functions
internal/middleware/ - HTTP middleware (auth, etc.)
internal/models/     - Data models and store interfaces
```

## Error Handling
- Explicit `(result, error)` return tuples throughout
- Errors propagated up to handlers for HTTP response formatting
- Fatal errors logged with `log.Fatalf()` at startup

## Logging
- Standard library `log` package
- `log.Printf()` for informational messages
- `log.Fatalf()` for fatal startup errors

## Import Order
1. Standard library
2. External packages
3. Internal packages

## Database Field Mapping
- DB snake_case → Go struct PascalCase via struct tags (e.g., `` `db:"product_id"` ``)
