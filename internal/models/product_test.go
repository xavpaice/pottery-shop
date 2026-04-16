package models

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	migrations "pottery-shop/internal/migrations"
)

var testDBURL string

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

	testDBURL, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic("failed to get connection string: " + err.Error())
	}

	// Run migrations once on shared container.
	// NOTE on goose.Up path: when using the internal/migrations package pattern
	// where migrations.FS embeds *.sql at the package root via //go:embed *.sql,
	// the correct goose.Up path is "." (root of the embedded FS). Using "migrations"
	// would silently find zero migration files and create no tables.
	pool, err := pgxpool.New(ctx, testDBURL)
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

func setupTestStore(t *testing.T) *ProductStore {
	t.Helper()
	if testDBURL == "" {
		t.Skip("testcontainers unavailable; skipping DB-dependent test")
	}
	pool, err := pgxpool.New(context.Background(), testDBURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() {
		db.Close()
		pool.Close()
	})

	// Truncate tables between tests — resets identity sequences for predictable IDs
	if _, err := db.Exec("TRUNCATE products, images RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	return NewProductStore(db)
}

func createSampleProduct(t *testing.T, store *ProductStore, title string, price float64) *Product {
	t.Helper()
	p := &Product{Title: title, Description: "A test product", Price: price}
	if err := store.Create(p, 0); err != nil {
		t.Fatalf("create product: %v", err)
	}
	return p
}

func TestCreateAndGetByID(t *testing.T) {
	store := setupTestStore(t)
	p := createSampleProduct(t, store, "Blue Mug", 29.99)

	if p.ID == 0 {
		t.Fatal("expected non-zero product ID after create")
	}

	got, err := store.GetByID(p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != "Blue Mug" {
		t.Errorf("title = %q, want %q", got.Title, "Blue Mug")
	}
	if got.Price != 29.99 {
		t.Errorf("price = %f, want %f", got.Price, 29.99)
	}
	if got.IsSold {
		t.Error("expected IsSold = false for new product")
	}
}

func TestGetByID_NotFound(t *testing.T) {
	store := setupTestStore(t)
	_, err := store.GetByID(999)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestUpdate(t *testing.T) {
	store := setupTestStore(t)
	p := createSampleProduct(t, store, "Vase", 50.00)

	p.Title = "Large Vase"
	p.Price = 65.00
	p.IsSold = true
	if err := store.Update(p); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.GetByID(p.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if got.Title != "Large Vase" {
		t.Errorf("title = %q, want %q", got.Title, "Large Vase")
	}
	if got.Price != 65.00 {
		t.Errorf("price = %f, want %f", got.Price, 65.00)
	}
	if !got.IsSold {
		t.Error("expected IsSold = true after update")
	}
}

func TestDelete(t *testing.T) {
	store := setupTestStore(t)
	p := createSampleProduct(t, store, "Bowl", 20.00)

	if err := store.Delete(p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.GetByID(p.ID)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows after delete, got %v", err)
	}
}

func TestDelete_NonExistent(t *testing.T) {
	store := setupTestStore(t)
	// Deleting a non-existent ID should not error
	if err := store.Delete(999); err != nil {
		t.Errorf("Delete non-existent: %v", err)
	}
}

func TestListAll(t *testing.T) {
	store := setupTestStore(t)
	createSampleProduct(t, store, "Mug", 25.00)
	createSampleProduct(t, store, "Plate", 35.00)

	products, err := store.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(products) != 2 {
		t.Errorf("ListAll count = %d, want 2", len(products))
	}
}

func TestListAvailable(t *testing.T) {
	store := setupTestStore(t)
	createSampleProduct(t, store, "Available Mug", 25.00)
	sold := createSampleProduct(t, store, "Sold Plate", 35.00)
	sold.IsSold = true
	store.Update(sold)

	products, err := store.ListAvailable()
	if err != nil {
		t.Fatalf("ListAvailable: %v", err)
	}
	if len(products) != 1 {
		t.Errorf("ListAvailable count = %d, want 1", len(products))
	}
	if len(products) > 0 && products[0].Title != "Available Mug" {
		t.Errorf("expected Available Mug, got %q", products[0].Title)
	}
}

func TestListSold(t *testing.T) {
	store := setupTestStore(t)
	createSampleProduct(t, store, "Unsold Bowl", 20.00)
	sold := createSampleProduct(t, store, "Sold Vase", 40.00)
	sold.IsSold = true
	store.Update(sold)

	products, err := store.ListSold()
	if err != nil {
		t.Fatalf("ListSold: %v", err)
	}
	if len(products) != 1 {
		t.Errorf("ListSold count = %d, want 1", len(products))
	}
	if len(products) > 0 && products[0].Title != "Sold Vase" {
		t.Errorf("expected Sold Vase, got %q", products[0].Title)
	}
}

func TestListAll_Empty(t *testing.T) {
	store := setupTestStore(t)
	products, err := store.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if products != nil {
		t.Errorf("expected nil for empty list, got %v", products)
	}
}

func TestAddImage(t *testing.T) {
	store := setupTestStore(t)
	p := createSampleProduct(t, store, "Mug", 25.00)

	img := &Image{ProductID: p.ID, Filename: "mug.jpg", ThumbnailFn: "mug_thumb.jpg", SortOrder: 1}
	if err := store.AddImage(img); err != nil {
		t.Fatalf("AddImage: %v", err)
	}
	if img.ID == 0 {
		t.Fatal("expected non-zero image ID after add")
	}
}

func TestGetImages(t *testing.T) {
	store := setupTestStore(t)
	p := createSampleProduct(t, store, "Mug", 25.00)

	store.AddImage(&Image{ProductID: p.ID, Filename: "a.jpg", SortOrder: 2})
	store.AddImage(&Image{ProductID: p.ID, Filename: "b.jpg", SortOrder: 1})

	images, err := store.GetImages(p.ID)
	if err != nil {
		t.Fatalf("GetImages: %v", err)
	}
	if len(images) != 2 {
		t.Fatalf("GetImages count = %d, want 2", len(images))
	}
	// Should be ordered by sort_order
	if images[0].Filename != "b.jpg" {
		t.Errorf("first image = %q, want b.jpg (lower sort_order)", images[0].Filename)
	}
}

func TestGetImages_Empty(t *testing.T) {
	store := setupTestStore(t)
	p := createSampleProduct(t, store, "Mug", 25.00)

	images, err := store.GetImages(p.ID)
	if err != nil {
		t.Fatalf("GetImages: %v", err)
	}
	if images != nil {
		t.Errorf("expected nil for no images, got %v", images)
	}
}

func TestDeleteImage(t *testing.T) {
	store := setupTestStore(t)
	p := createSampleProduct(t, store, "Mug", 25.00)

	img := &Image{ProductID: p.ID, Filename: "mug.jpg", ThumbnailFn: "mug_thumb.jpg", SortOrder: 1}
	store.AddImage(img)

	deleted, err := store.DeleteImage(img.ID)
	if err != nil {
		t.Fatalf("DeleteImage: %v", err)
	}
	if deleted.Filename != "mug.jpg" {
		t.Errorf("deleted filename = %q, want mug.jpg", deleted.Filename)
	}

	images, _ := store.GetImages(p.ID)
	if len(images) != 0 {
		t.Errorf("expected 0 images after delete, got %d", len(images))
	}
}

func TestDeleteImage_NotFound(t *testing.T) {
	store := setupTestStore(t)
	_, err := store.DeleteImage(999)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestCountImages(t *testing.T) {
	store := setupTestStore(t)
	p := createSampleProduct(t, store, "Mug", 25.00)

	count, err := store.CountImages(p.ID)
	if err != nil {
		t.Fatalf("CountImages: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	store.AddImage(&Image{ProductID: p.ID, Filename: "a.jpg", SortOrder: 1})
	store.AddImage(&Image{ProductID: p.ID, Filename: "b.jpg", SortOrder: 2})

	count, err = store.CountImages(p.ID)
	if err != nil {
		t.Fatalf("CountImages: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestGetByID_IncludesImages(t *testing.T) {
	store := setupTestStore(t)
	p := createSampleProduct(t, store, "Mug", 25.00)
	store.AddImage(&Image{ProductID: p.ID, Filename: "front.jpg", SortOrder: 1})

	got, err := store.GetByID(p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(got.Images) != 1 {
		t.Errorf("expected 1 image, got %d", len(got.Images))
	}
}
