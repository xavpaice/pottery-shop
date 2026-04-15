package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	migrations "pottery-shop/internal/migrations"

	"pottery-shop/internal/handlers"
	"pottery-shop/internal/middleware"
	"pottery-shop/internal/models"
)

func main() {
	// Config from env with defaults
	port := envOr("PORT", "8080")
	baseURL := envOr("BASE_URL", "http://localhost:8080")
	adminUser := envOr("ADMIN_USER", "admin")
	adminPass := envOr("ADMIN_PASS", "changeme")
	sessionSecret := envOr("SESSION_SECRET", "change-this-to-a-random-string-at-least-32-chars")
	uploadDir := envOr("UPLOAD_DIR", "uploads")
	thumbDir := filepath.Join(uploadDir, "thumbnails")

	// Ensure directories exist
	os.MkdirAll(uploadDir, 0755)
	os.MkdirAll(thumbDir, 0755)

	// Database
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL must be set")
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	// Run migrations
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("Failed to set goose dialect: %v", err)
	}
	if err := goose.Up(db, "."); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	store := models.NewProductStore(db)

	// Templates
	funcMap := template.FuncMap{
		"lower": strings.ToLower,
	}

	publicTemplates := template.Must(
		template.New("").Funcs(funcMap).ParseGlob("templates/partials/*.html"),
	)
	template.Must(publicTemplates.ParseGlob("templates/*.html"))

	adminTemplates := template.Must(
		template.New("").Funcs(funcMap).ParseGlob("templates/partials/*.html"),
	)
	template.Must(adminTemplates.ParseGlob("templates/admin/*.html"))

	// Session manager
	sessionMgr := middleware.NewSessionManager(sessionSecret)

	// Handlers
	config := &handlers.Config{
		SMTPHost:   os.Getenv("SMTP_HOST"),
		SMTPPort:   envOr("SMTP_PORT", "587"),
		SMTPUser:   os.Getenv("SMTP_USER"),
		SMTPPass:   os.Getenv("SMTP_PASS"),
		SMTPFrom:   os.Getenv("SMTP_FROM"),
		OrderEmail: envOr("ORDER_EMAIL", "xavpaice@gmail.com"),
		BaseURL:    baseURL,
	}

	publicHandler := &handlers.PublicHandler{
		Store:     store,
		Templates: publicTemplates,
		Session:   sessionMgr,
		Config:    config,
	}

	adminHandler := &handlers.AdminHandler{
		Store:     store,
		Templates: adminTemplates,
		UploadDir: uploadDir,
		ThumbDir:  thumbDir,
	}

	// Mux
	mux := http.NewServeMux()

	// Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))

	// Health endpoints (registered before / catch-all)
	mux.HandleFunc("/healthz", handlers.Healthz)
	mux.HandleFunc("/readyz", handlers.ReadyzHandler(db))

	// Public routes
	mux.HandleFunc("/", publicHandler.Home)
	mux.HandleFunc("/gallery", publicHandler.Gallery)
	mux.HandleFunc("/product/", publicHandler.ProductDetail)
	mux.HandleFunc("/cart", publicHandler.ViewCart)
	mux.HandleFunc("/cart/add", publicHandler.AddToCart)
	mux.HandleFunc("/cart/remove", publicHandler.RemoveFromCart)
	mux.HandleFunc("/order", publicHandler.PlaceOrder)
	mux.HandleFunc("/order-confirmed", publicHandler.OrderConfirmed)

	// Admin routes (behind basic auth)
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/admin", adminHandler.Dashboard)
	adminMux.HandleFunc("/admin/", adminHandler.Dashboard)
	adminMux.HandleFunc("/admin/products/new", adminHandler.NewProduct)
	adminMux.HandleFunc("/admin/products/create", adminHandler.CreateProduct)
	adminMux.HandleFunc("/admin/products/update", adminHandler.UpdateProduct)
	adminMux.HandleFunc("/admin/products/delete", adminHandler.DeleteProduct)
	adminMux.HandleFunc("/admin/products/toggle-sold", adminHandler.ToggleSold)
	adminMux.HandleFunc("/admin/products/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/edit") {
			adminHandler.EditProduct(w, r)
		} else {
			http.NotFound(w, r)
		}
	})
	adminMux.HandleFunc("/admin/images/delete", adminHandler.DeleteImage)

	authAdmin := middleware.BasicAuth(adminUser, adminPass, adminMux)
	mux.Handle("/admin/", authAdmin)
	mux.Handle("/admin", authAdmin)

	// Wrap everything in session middleware
	handler := sessionMgr.Middleware(mux)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Clay.nz starting on %s", addr)
	log.Printf("   Public:  %s", baseURL)
	log.Printf("   Admin:   %s/admin  (user: %s)", baseURL, adminUser)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
