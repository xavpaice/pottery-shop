---
phase: 01-go-build
reviewed: 2026-04-13T00:00:00Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - Dockerfile
  - Makefile
  - cmd/server/main.go
  - go.mod
  - go.sum
  - internal/handlers/public_test.go
  - internal/migrations/00001_initial_schema.sql
  - internal/migrations/migrations.go
  - internal/models/product.go
  - internal/models/product_test.go
findings:
  critical: 2
  warning: 3
  info: 4
  total: 9
status: issues_found
---

# Phase 01: Code Review Report

**Reviewed:** 2026-04-13
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

This phase delivers the core SQLite-to-Postgres migration: a CGO-free Go binary, pgx/v5 connection pool, goose migrations, and testcontainers-based integration tests. The overall structure is clean and well-organized. Two critical issues exist: a hardcoded fallback admin password and silently ignored directory-creation errors at startup. Three warnings cover a silent DB error discard in `listProducts`, a non-atomic delete-image operation, and a hardcoded default session secret. Four informational items round out the review.

## Critical Issues

### CR-01: Hardcoded Default Admin Password

**File:** `cmd/server/main.go:29`

**Issue:** `adminPass` falls back to the literal string `"changeme"` when `ADMIN_PASS` is not set. Anyone who knows the default can authenticate to the admin area without any configuration error signaling the omission.

**Fix:** Fail fast at startup if `ADMIN_PASS` is absent rather than silently using a known-weak default:
```go
adminPass := os.Getenv("ADMIN_PASS")
if adminPass == "" {
    log.Fatal("ADMIN_PASS must be set")
}
```

---

### CR-02: `os.MkdirAll` Errors Silently Ignored at Startup

**File:** `cmd/server/main.go:35-36`

**Issue:** Both `os.MkdirAll` calls discard their error return values. If the process lacks permission to create the upload or thumbnail directories, the server starts successfully, then fails at runtime when the first image upload is attempted — producing a confusing error far from the root cause.

**Fix:** Check and fatal on directory creation errors:
```go
if err := os.MkdirAll(uploadDir, 0755); err != nil {
    log.Fatalf("Failed to create upload dir %q: %v", uploadDir, err)
}
if err := os.MkdirAll(thumbDir, 0755); err != nil {
    log.Fatalf("Failed to create thumb dir %q: %v", thumbDir, err)
}
```

---

## Warnings

### WR-01: `GetImages` Errors Silently Discarded in `listProducts`

**File:** `internal/models/product.go:95`

**Issue:** `p.Images, _ = s.GetImages(p.ID)` throws away the error. A transient DB error, a connection reset, or a schema mismatch will cause every product to silently have an empty image slice. Callers have no way to distinguish "no images" from "images failed to load."

**Fix:** Propagate the error:
```go
p.Images, err = s.GetImages(p.ID)
if err != nil {
    return nil, err
}
```

---

### WR-02: Non-Atomic `DeleteImage` — SELECT then DELETE Race

**File:** `internal/models/product.go:131-139`

**Issue:** `DeleteImage` executes a SELECT to retrieve the image record and then a separate DELETE. Between these two statements, a concurrent request could delete the same row. The SELECT would succeed (returning valid data), but the subsequent DELETE would affect zero rows and return no error — leaving the caller believing it succeeded when the row was already gone and the returned `*Image` data is stale. In the current single-process deployment this is low-probability, but the pattern is incorrect.

**Fix:** Combine into a single atomic statement using `RETURNING`:
```go
func (s *ProductStore) DeleteImage(id int64) (*Image, error) {
    img := &Image{}
    err := s.DB.QueryRow(
        `DELETE FROM images WHERE id=$1 RETURNING id, product_id, filename, thumbnail_fn`,
        id,
    ).Scan(&img.ID, &img.ProductID, &img.Filename, &img.ThumbnailFn)
    if err != nil {
        return nil, err // sql.ErrNoRows if not found
    }
    return img, nil
}
```

---

### WR-03: Hardcoded Default Session Secret

**File:** `cmd/server/main.go:30`

**Issue:** `sessionSecret` falls back to the well-known string `"change-this-to-a-random-string-at-least-32-chars"`. Any deployment that omits `SESSION_SECRET` uses this value, allowing an attacker who knows the default to forge valid session cookies (the session is HMAC-SHA256 signed).

**Fix:** Fail fast if `SESSION_SECRET` is absent, consistent with how `DATABASE_URL` is handled:
```go
sessionSecret := os.Getenv("SESSION_SECRET")
if sessionSecret == "" {
    log.Fatal("SESSION_SECRET must be set")
}
```

---

## Info

### IN-01: `go 1.26` Module Directive References a Future Go Version

**File:** `go.mod:3`

**Issue:** `go 1.26` is declared in the module directive. As of this review, Go 1.26 does not exist (latest stable is 1.23.x). This may cause issues with some toolchains that validate the version field strictly, and is inconsistent with the `golang:1.26-alpine` builder image tag in the Dockerfile (which itself would not be pullable today).

**Fix:** Align the `go` directive with the actual installed toolchain version once Go 1.26 is released, or correct to the currently intended version (e.g., `go 1.23`).

---

### IN-02: Hardcoded Personal Email as `ORDER_EMAIL` Default

**File:** `cmd/server/main.go:89`

**Issue:** `envOr("ORDER_EMAIL", "xavpaice@gmail.com")` embeds a personal email address as the compiled-in fallback. Deployments that omit `ORDER_EMAIL` will send order notifications to this address silently.

**Fix:** Either remove the fallback and require `ORDER_EMAIL` to be set, or use an obviously invalid placeholder:
```go
orderEmail := os.Getenv("ORDER_EMAIL")
if orderEmail == "" {
    log.Fatal("ORDER_EMAIL must be set")
}
```

---

### IN-03: `store.Update` Return Value Ignored in Test

**File:** `internal/models/product_test.go:195`

**Issue:** In `TestListAvailable`, `store.Update(sold)` is called without checking the error. If `Update` fails, the test proceeds as if the product was marked sold and will produce a misleading failure.

**Fix:**
```go
if err := store.Update(sold); err != nil {
    t.Fatalf("Update sold product: %v", err)
}
```
The same pattern appears in `TestListSold` at line 213.

---

### IN-04: Test Hardcodes Product ID `1` Instead of Using Created Product's ID

**File:** `internal/handlers/public_test.go:236`

**Issue:** `TestProductDetail_Found` requests `/product/1` by hardcoded path rather than constructing the path from `p.ID`. This works because `TRUNCATE ... RESTART IDENTITY` resets the sequence before each test, but it is fragile: if test parallelism, identity sequence behavior, or test ordering changes, the test will return 404 and the failure message will be misleading.

**Fix:**
```go
req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/product/%d", p.ID), nil)
```

---

_Reviewed: 2026-04-13_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
