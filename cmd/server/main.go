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

	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	migrations "pottery-shop/internal/migrations"

	"pottery-shop/internal/handlers"
	"pottery-shop/internal/metrics"
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

	// Validate license before proceeding
	sdkService := envOr("REPLICATED_SDK_SERVICE", "clay-sdk")
	if err := metrics.ValidateLicense(sdkService); err != nil {
		log.Fatalf("License validation failed: %v", err)
	}

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
	sellerStore := models.NewSellerStore(pool)

	// Bootstrap admin seller from env vars if no admin exists yet
	ctx := context.Background()
	exists, err := sellerStore.AdminExists(ctx)
	if err != nil {
		log.Fatalf("Failed to check admin existence: %v", err)
	}
	if !exists {
		if adminUser != "" && adminPass != "" {
			if err := sellerStore.CreateAdmin(ctx, adminUser, adminPass); err != nil {
				log.Fatalf("bootstrap admin seller: %v", err)
			}
			log.Printf("bootstrap: created admin seller %s", adminUser)
		}
	}

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
		Sellers:   sellerStore,
		Templates: publicTemplates,
		Session:   sessionMgr,
		Config:    config,
	}

	sdkService := envOr("REPLICATED_SDK_SERVICE", "clay-sdk")
	updateChecker := metrics.NewUpdateChecker(sdkService, 1*time.Hour)

	adminHandler := &handlers.AdminHandler{
		Store:         store,
		Sellers:       sellerStore,
		Templates:     adminTemplates,
		UploadDir:     uploadDir,
		ThumbDir:      thumbDir,
		UpdateChecker: updateChecker,
	}

	// Check license field for firing logs entitlement, fall back to env var
	envFallback := envOr("FEATURE_FIRING_LOGS_ENABLED", "true") != "false"
	firingLogsEnabled := metrics.CheckLicenseFieldBool(sdkService, "enableFiringLogs", envFallback)
	log.Printf("Firing logs enabled: %v", firingLogsEnabled)

	authHandler := handlers.NewAuthHandler(sellerStore, store, sessionMgr, publicTemplates, config, uploadDir, thumbDir)
	authHandler.FiringLogsEnabled = firingLogsEnabled

	var firingLogHandler *handlers.FiringLogHandler
	if firingLogsEnabled {
		firingLogStore := models.NewFiringLogStore(pool)
		firingLogHandler = handlers.NewFiringLogHandler(firingLogStore, sessionMgr, publicTemplates)
	}

	// Mux
	mux := http.NewServeMux()

	// Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))

	// Health endpoints (registered before / catch-all)
	mux.HandleFunc("/healthz", handlers.Healthz)
	mux.HandleFunc("/readyz", handlers.ReadyzHandler(db))

	// Seller auth routes (no Basic Auth — cookie-session based)
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			authHandler.Login(w, r)
		} else {
			authHandler.ShowLogin(w, r)
		}
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			authHandler.Register(w, r)
		} else {
			authHandler.ShowRegister(w, r)
		}
	})
	mux.HandleFunc("/logout", authHandler.Logout)
	mux.HandleFunc("/dashboard", authHandler.Dashboard)

	// Seller dashboard firing log routes (guarded by RequireSeller at registration)
	if firingLogsEnabled {
		mux.HandleFunc("/dashboard/firings", authHandler.RequireSeller(firingLogHandler.List))
		mux.HandleFunc("/dashboard/firings/new", authHandler.RequireSeller(firingLogHandler.New))
		mux.HandleFunc("/dashboard/firings/create", authHandler.RequireSeller(firingLogHandler.Create))
		mux.HandleFunc("/dashboard/firings/", func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.HasSuffix(path, "/edit") {
				authHandler.RequireSeller(firingLogHandler.Edit)(w, r)
			} else if strings.HasSuffix(path, "/update") {
				authHandler.RequireSeller(firingLogHandler.Update)(w, r)
			} else if strings.HasSuffix(path, "/delete") {
				authHandler.RequireSeller(firingLogHandler.Delete)(w, r)
			} else {
				authHandler.RequireSeller(firingLogHandler.View)(w, r)
			}
		})
	}

	// Seller dashboard product routes (guarded by requireSeller inside each handler)
	mux.HandleFunc("/dashboard/products", authHandler.DashboardProducts)
	mux.HandleFunc("/dashboard/products/new", authHandler.DashboardNewProduct)
	mux.HandleFunc("/dashboard/products/create", authHandler.DashboardCreateProduct)
	mux.HandleFunc("/dashboard/products/update", authHandler.DashboardUpdateProduct)
	mux.HandleFunc("/dashboard/products/delete", authHandler.DashboardDeleteProduct)
	mux.HandleFunc("/dashboard/products/toggle-sold", authHandler.DashboardToggleSold)
	mux.HandleFunc("/dashboard/products/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/edit") {
			authHandler.DashboardEditProduct(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	// JSON API routes (auth enforced inside handler — returns JSON errors, not HTML redirects)
	if firingLogsEnabled {
		mux.HandleFunc("GET /api/firings/{id}/readings", firingLogHandler.ReadingsAPI)
	}

	// Public routes
	mux.HandleFunc("/", publicHandler.Home)
	mux.HandleFunc("/gallery", publicHandler.Gallery)
	mux.HandleFunc("/product/", publicHandler.ProductDetail)
	mux.HandleFunc("GET /seller/{id}", publicHandler.SellerProfile)
	mux.HandleFunc("/cart", publicHandler.ViewCart)
	mux.HandleFunc("/cart/add", publicHandler.AddToCart)
	mux.HandleFunc("/cart/remove", publicHandler.RemoveFromCart)
	mux.HandleFunc("/order", publicHandler.PlaceOrder)
	mux.HandleFunc("/order-confirmed", publicHandler.OrderConfirmed)

	// Seller approval route — token IS the auth, no Basic Auth required
	// Token expiry is a future enhancement.
	// GET only: this is a link in the approval email; POST is handled by adminMux.
	mux.HandleFunc("GET /admin/sellers/approve", authHandler.ApproveSellerByToken)

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
	adminMux.HandleFunc("/admin/sellers", adminHandler.SellerList)
	adminMux.HandleFunc("POST /admin/sellers/approve", adminHandler.ApproveSeller)
	adminMux.HandleFunc("POST /admin/sellers/reject", adminHandler.RejectSeller)

	authAdmin := middleware.BasicAuth(adminUser, adminPass, adminMux)
	mux.Handle("/admin/", authAdmin)
	mux.Handle("/admin", authAdmin)

	// Wrap everything in session middleware
	handler := sessionMgr.Middleware(mux)

	// Custom metrics reporter -- posts counts to Replicated SDK every 4 hours
	metricsReporter := metrics.NewReporter(db, pool, sdkService, 4*time.Hour)
	go metricsReporter.Run(context.Background())
	go updateChecker.Run(context.Background())

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
