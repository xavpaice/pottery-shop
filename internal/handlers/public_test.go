package handlers

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	migrations "pottery-shop/internal/migrations"
	"pottery-shop/internal/middleware"
	"pottery-shop/internal/models"
)

const templatesDir = "../../templates"

var handlersTestDBURL string

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	if err != nil {
		// Docker unavailable (e.g. CI without bridge network) -- run non-DB tests only.
		fmt.Fprintf(os.Stderr, "SKIP: testcontainers unavailable (%v); DB-dependent tests will be skipped\n", err)
		os.Exit(m.Run())
	}
	defer pgContainer.Terminate(ctx)

	handlersTestDBURL, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic("failed to get connection string: " + err.Error())
	}

	// Run migrations once on shared container.
	pool, err := pgxpool.New(ctx, handlersTestDBURL)
	if err != nil {
		panic("failed to create pool: " + err.Error())
	}
	db := stdlib.OpenDBFromPool(pool)

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		panic("goose dialect: " + err.Error())
	}
	if err := goose.Up(db, "."); err != nil {
		panic("goose up: " + err.Error())
	}
	db.Close()
	pool.Close()

	os.Exit(m.Run())
}

func setupTestEnv(t *testing.T) (*PublicHandler, *middleware.SessionManager) {
	t.Helper()

	if handlersTestDBURL == "" {
		t.Skip("testcontainers unavailable; skipping DB-dependent test")
	}

	pool, err := pgxpool.New(context.Background(), handlersTestDBURL)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() {
		db.Close()
		pool.Close()
	})

	// Truncate tables between tests
	if _, err := db.Exec("TRUNCATE products, images RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	store := models.NewProductStore(db)

	funcMap := template.FuncMap{
		"lower":             strings.ToLower,
		"customLogoEnabled": func() bool { return false },
	}

	publicTemplates := template.Must(
		template.New("").Funcs(funcMap).ParseGlob(templatesDir + "/partials/*.html"),
	)
	template.Must(publicTemplates.ParseGlob(templatesDir + "/*.html"))

	sm := middleware.NewSessionManager("test-secret-key-for-testing")

	handler := &PublicHandler{
		Store:     store,
		Templates: publicTemplates,
		Session:   sm,
		Config: &Config{
			BaseURL:    "http://localhost:8080",
			OrderEmail: "test@example.com",
		},
	}

	return handler, sm
}

func createTestProduct(t *testing.T, store *models.ProductStore, title string, price float64, sold bool) *models.Product {
	t.Helper()
	p := &models.Product{
		Title:       title,
		Description: "A test product",
		Price:       price,
		IsSold:      sold,
	}
	if err := store.Create(p, 0); err != nil {
		t.Fatalf("failed to create product: %v", err)
	}
	return p
}

func createTestImage(t *testing.T, store *models.ProductStore, productID int64) {
	t.Helper()
	img := &models.Image{
		ProductID:   productID,
		Filename:    "test.jpg",
		ThumbnailFn: "thumb_test.jpg",
		SortOrder:   0,
	}
	if err := store.AddImage(img); err != nil {
		t.Fatalf("failed to add image: %v", err)
	}
}

// executeWithSession wraps handler in session middleware and returns recorder
func executeWithSession(sm *middleware.SessionManager, handler http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	wrapped := sm.Middleware(http.HandlerFunc(handler))
	wrapped.ServeHTTP(rec, req)
	return rec
}

// extractSessionCookie gets the session cookie value from a response.
// We read from rec.Header() directly because ResponseRecorder.Result()
// snapshots headers at WriteHeader() time, but the session middleware
// sets the cookie AFTER the handler returns (after any redirect WriteHeader).
func extractSessionCookie(rec *httptest.ResponseRecorder) string {
	for _, line := range rec.Header()["Set-Cookie"] {
		// Parse "pottery_session=VALUE; Path=/; ..."
		if strings.HasPrefix(line, "pottery_session=") {
			val := strings.TrimPrefix(line, "pottery_session=")
			if idx := strings.Index(val, ";"); idx >= 0 {
				val = val[:idx]
			}
			return val
		}
	}
	return ""
}

// addSessionCookie adds a session cookie from a previous response to a new request
func addSessionCookie(req *http.Request, cookieValue string) {
	if cookieValue != "" {
		req.AddCookie(&http.Cookie{Name: "pottery_session", Value: cookieValue})
	}
}

// --- Home ---

func TestHome_EmptyProducts(t *testing.T) {
	h, sm := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := executeWithSession(sm, h.Home, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Clay.nz") || !strings.Contains(body, "html") {
		t.Error("expected rendered HTML page")
	}
}

func TestHome_WithProducts(t *testing.T) {
	h, sm := setupTestEnv(t)

	createTestProduct(t, h.Store, "Blue Mug", 35.00, false)
	createTestProduct(t, h.Store, "Red Bowl", 50.00, false)
	createTestProduct(t, h.Store, "Sold Vase", 100.00, true) // should not appear

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := executeWithSession(sm, h.Home, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Blue Mug") {
		t.Error("expected 'Blue Mug' in response")
	}
	if !strings.Contains(body, "Red Bowl") {
		t.Error("expected 'Red Bowl' in response")
	}
}

func TestHome_NotFoundForOtherPaths(t *testing.T) {
	h, sm := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := executeWithSession(sm, h.Home, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// --- ProductDetail ---

func TestProductDetail_Found(t *testing.T) {
	h, sm := setupTestEnv(t)

	p := createTestProduct(t, h.Store, "Test Mug", 42.00, false)
	createTestImage(t, h.Store, p.ID)

	req := httptest.NewRequest(http.MethodGet, "/product/1", nil)
	rec := executeWithSession(sm, h.ProductDetail, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Test Mug") {
		t.Error("expected product title in response")
	}
}

func TestProductDetail_NotFound(t *testing.T) {
	h, sm := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/product/999", nil)
	rec := executeWithSession(sm, h.ProductDetail, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestProductDetail_InvalidID(t *testing.T) {
	h, sm := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/product/abc", nil)
	rec := executeWithSession(sm, h.ProductDetail, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// --- AddToCart ---

func TestAddToCart_Success(t *testing.T) {
	h, sm := setupTestEnv(t)

	p := createTestProduct(t, h.Store, "Cart Mug", 30.00, false)
	createTestImage(t, h.Store, p.ID)

	form := url.Values{"product_id": {fmt.Sprintf("%d", p.ID)}}
	req := httptest.NewRequest(http.MethodPost, "/cart/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "/")

	rec := executeWithSession(sm, h.AddToCart, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != "/" {
		t.Errorf("expected redirect to '/', got %q", loc)
	}
}

func TestAddToCart_SoldProduct(t *testing.T) {
	h, sm := setupTestEnv(t)

	p := createTestProduct(t, h.Store, "Sold Item", 50.00, true)

	form := url.Values{"product_id": {fmt.Sprintf("%d", p.ID)}}
	req := httptest.NewRequest(http.MethodPost, "/cart/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := executeWithSession(sm, h.AddToCart, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestAddToCart_GetMethod(t *testing.T) {
	h, sm := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/cart/add", nil)
	rec := executeWithSession(sm, h.AddToCart, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestAddToCart_InvalidProductID(t *testing.T) {
	h, sm := setupTestEnv(t)

	form := url.Values{"product_id": {"abc"}}
	req := httptest.NewRequest(http.MethodPost, "/cart/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := executeWithSession(sm, h.AddToCart, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// --- RemoveFromCart ---

func TestRemoveFromCart_Success(t *testing.T) {
	h, sm := setupTestEnv(t)

	p := createTestProduct(t, h.Store, "Remove Mug", 30.00, false)
	createTestImage(t, h.Store, p.ID)

	// First add to cart
	form := url.Values{"product_id": {fmt.Sprintf("%d", p.ID)}}
	req := httptest.NewRequest(http.MethodPost, "/cart/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "/")
	rec := executeWithSession(sm, h.AddToCart, req)
	sessionCookie := extractSessionCookie(rec)

	// Then remove from cart
	form = url.Values{"product_id": {fmt.Sprintf("%d", p.ID)}}
	req = httptest.NewRequest(http.MethodPost, "/cart/remove", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSessionCookie(req, sessionCookie)
	rec = executeWithSession(sm, h.RemoveFromCart, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/cart" {
		t.Errorf("expected redirect to '/cart', got %q", loc)
	}
}

func TestRemoveFromCart_GetMethod(t *testing.T) {
	h, sm := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/cart/remove", nil)
	rec := executeWithSession(sm, h.RemoveFromCart, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

// --- ViewCart ---

func TestViewCart_Empty(t *testing.T) {
	h, sm := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/cart", nil)
	rec := executeWithSession(sm, h.ViewCart, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestViewCart_WithItems(t *testing.T) {
	h, sm := setupTestEnv(t)

	p := createTestProduct(t, h.Store, "Cart Bowl", 45.00, false)
	createTestImage(t, h.Store, p.ID)

	// Add to cart first
	form := url.Values{"product_id": {fmt.Sprintf("%d", p.ID)}}
	req := httptest.NewRequest(http.MethodPost, "/cart/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "/")
	rec := executeWithSession(sm, h.AddToCart, req)
	sessionCookie := extractSessionCookie(rec)

	// View cart
	req = httptest.NewRequest(http.MethodGet, "/cart", nil)
	addSessionCookie(req, sessionCookie)
	rec = executeWithSession(sm, h.ViewCart, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Cart Bowl") {
		t.Error("expected 'Cart Bowl' in cart view")
	}
}

// --- PlaceOrder ---

func TestPlaceOrder_EmptyCart(t *testing.T) {
	h, sm := setupTestEnv(t)

	form := url.Values{
		"name":  {"Test User"},
		"email": {"test@example.com"},
	}
	req := httptest.NewRequest(http.MethodPost, "/order", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := executeWithSession(sm, h.PlaceOrder, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/cart" {
		t.Errorf("expected redirect to '/cart', got %q", loc)
	}
}

func TestPlaceOrder_MissingNameEmail(t *testing.T) {
	h, sm := setupTestEnv(t)

	p := createTestProduct(t, h.Store, "Order Mug", 30.00, false)
	createTestImage(t, h.Store, p.ID)

	// Add to cart
	form := url.Values{"product_id": {fmt.Sprintf("%d", p.ID)}}
	req := httptest.NewRequest(http.MethodPost, "/cart/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "/")
	rec := executeWithSession(sm, h.AddToCart, req)
	sessionCookie := extractSessionCookie(rec)

	// Place order without name/email
	form = url.Values{}
	req = httptest.NewRequest(http.MethodPost, "/order", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSessionCookie(req, sessionCookie)
	rec = executeWithSession(sm, h.PlaceOrder, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/cart" {
		t.Errorf("expected redirect to '/cart', got %q", loc)
	}
}

func TestPlaceOrder_Success(t *testing.T) {
	h, sm := setupTestEnv(t)

	p := createTestProduct(t, h.Store, "Order Bowl", 55.00, false)
	createTestImage(t, h.Store, p.ID)

	// Add to cart
	form := url.Values{"product_id": {fmt.Sprintf("%d", p.ID)}}
	req := httptest.NewRequest(http.MethodPost, "/cart/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "/")
	rec := executeWithSession(sm, h.AddToCart, req)
	sessionCookie := extractSessionCookie(rec)

	// Place order with valid name/email
	form = url.Values{
		"name":    {"Jane Potter"},
		"email":   {"jane@example.com"},
		"message": {"I love pottery!"},
	}
	req = httptest.NewRequest(http.MethodPost, "/order", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSessionCookie(req, sessionCookie)
	rec = executeWithSession(sm, h.PlaceOrder, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/order-confirmed" {
		t.Errorf("expected redirect to '/order-confirmed', got %q", loc)
	}
}

func TestPlaceOrder_GetMethod(t *testing.T) {
	h, sm := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/order", nil)
	rec := executeWithSession(sm, h.PlaceOrder, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

// --- Gallery ---

func TestGallery_Empty(t *testing.T) {
	h, sm := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/gallery", nil)
	rec := executeWithSession(sm, h.Gallery, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestGallery_WithSoldProducts(t *testing.T) {
	h, sm := setupTestEnv(t)

	createTestProduct(t, h.Store, "Available Pot", 30.00, false) // should not appear
	p := createTestProduct(t, h.Store, "Sold Plate", 60.00, true)
	createTestImage(t, h.Store, p.ID)

	req := httptest.NewRequest(http.MethodGet, "/gallery", nil)
	rec := executeWithSession(sm, h.Gallery, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Sold Plate") {
		t.Error("expected 'Sold Plate' in gallery")
	}
}

// --- OrderConfirmed ---

func TestOrderConfirmed(t *testing.T) {
	h, sm := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/order-confirmed", nil)
	rec := executeWithSession(sm, h.OrderConfirmed, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
