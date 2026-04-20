package handlers

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"

	"pottery-shop/internal/metrics"
	"pottery-shop/internal/middleware"
	"pottery-shop/internal/models"
)

// AuthHandler handles seller authentication: login, register, logout, dashboard, and approval.
type AuthHandler struct {
	sellers          *models.SellerStore
	products         *models.ProductStore
	sessions         *middleware.SessionManager
	templates        *template.Template
	config           *Config
	uploadDir        string
	thumbDir         string
	FiringLogs       *metrics.FeatureChecker
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(sellers *models.SellerStore, products *models.ProductStore, sessions *middleware.SessionManager, templates *template.Template, config *Config, uploadDir, thumbDir string) *AuthHandler {
	return &AuthHandler{
		sellers:   sellers,
		products:  products,
		sessions:  sessions,
		templates: templates,
		config:    config,
		uploadDir: uploadDir,
		thumbDir:  thumbDir,
	}
}

// ShowLogin handles GET /login — renders the login form.
func (h *AuthHandler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	data := map[string]interface{}{
		"PageTitle": "Seller Login",
		"Flash":     session.Flash,
	}
	session.Flash = ""
	h.render(w, "login.html", data)
}

// Login handles POST /login — validates credentials and establishes a seller session.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	session := middleware.GetSession(r)

	seller, err := h.sellers.GetByEmail(r.Context(), email)
	if err != nil {
		log.Printf("Login: error fetching seller by email: %v", err)
		h.renderLoginError(w, r, "Invalid email or password")
		return
	}
	if seller == nil || !h.sellers.CheckPassword(seller, password) {
		log.Printf("Login: failed attempt for email=%s", email)
		h.renderLoginError(w, r, "Invalid email or password")
		return
	}
	if !seller.IsActive {
		log.Printf("Login: inactive account login attempt: email=%s id=%d", seller.Email, seller.ID)
		h.renderLoginError(w, r, "Your account is pending approval. You will be notified when it is approved.")
		return
	}

	log.Printf("Login: seller authenticated: name=%q email=%s id=%d", seller.Name, seller.Email, seller.ID)
	session.SellerID = seller.ID
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// ShowRegister handles GET /register — renders the registration form.
func (h *AuthHandler) ShowRegister(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	data := map[string]interface{}{
		"PageTitle": "Seller Registration",
		"Flash":     session.Flash,
	}
	session.Flash = ""
	h.render(w, "register.html", data)
}

// Register handles POST /register — creates a new pending seller account.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	name := strings.TrimSpace(r.FormValue("name"))

	if email == "" || password == "" || name == "" {
		h.renderRegisterError(w, r, "Email, password, and name are required")
		return
	}
	if len(password) < 8 {
		h.renderRegisterError(w, r, "Password must be at least 8 characters")
		return
	}

	seller, err := h.sellers.Create(r.Context(), email, password, name)
	if err != nil {
		log.Printf("Register: failed to create account for %s: %v", email, err)
		// Check for duplicate email (unique constraint violation)
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			h.renderRegisterError(w, r, "An account with that email already exists")
			return
		}
		http.Error(w, "Registration failed, please try again", http.StatusInternalServerError)
		return
	}

	log.Printf("Register: new seller account created: name=%q email=%s id=%d", seller.Name, seller.Email, seller.ID)

	// Send approval email — failure is logged but does not block registration.
	if err := h.sendApprovalEmail(seller.ApprovalToken); err != nil {
		log.Printf("Register: approval email failed for %s: %v", seller.Email, err)
	}

	session := middleware.GetSession(r)
	session.Flash = "Registration submitted — please await admin approval"
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// Logout handles POST /logout — clears the seller session and redirects to home.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	session := middleware.GetSession(r)
	session.SellerID = 0
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Dashboard handles GET /dashboard — seller landing page, requires active session.
func (h *AuthHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	h.requireSeller(h.dashboard)(w, r)
}

func (h *AuthHandler) dashboard(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)

	seller, err := h.sellers.GetByID(r.Context(), session.SellerID)
	if err != nil || seller == nil {
		session.SellerID = 0
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := map[string]interface{}{
		"PageTitle":         "Seller Dashboard",
		"Seller":            seller,
		"Flash":             session.Flash,
		"FiringLogsEnabled": h.FiringLogs != nil && h.FiringLogs.Enabled(),
	}
	session.Flash = ""
	h.render(w, "dashboard.html", data)
}

// ApproveSellerByToken handles GET /admin/sellers/approve?token=X.
// No Basic Auth is required on this route — the token itself is the auth credential.
// Token expiry is a future enhancement.
func (h *AuthHandler) ApproveSellerByToken(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing approval token", http.StatusBadRequest)
		return
	}

	seller, err := h.sellers.GetByApprovalToken(r.Context(), token)
	if err != nil {
		log.Printf("ApproveSellerByToken: error fetching seller by token: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if seller == nil {
		http.Error(w, "Invalid or expired approval token", http.StatusNotFound)
		return
	}

	if err := h.sellers.Approve(r.Context(), token); err != nil {
		log.Printf("ApproveSellerByToken: error approving seller %d: %v", seller.ID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("ApproveSellerByToken: seller approved: name=%q email=%s id=%d", seller.Name, seller.Email, seller.ID)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html><html><body><p>Seller <strong>%s</strong> approved. They can now log in.</p></body></html>`,
		template.HTMLEscapeString(seller.Name))
}

// RequireSeller is the exported form of requireSeller, allowing route registration
// in main.go to wrap FiringLogHandler (and future handlers) with the seller guard.
func (h *AuthHandler) RequireSeller(next http.HandlerFunc) http.HandlerFunc {
	return h.requireSeller(next)
}

// requireSeller is a middleware guard that redirects to /login if the seller
// is not authenticated or their account is not active.
func (h *AuthHandler) requireSeller(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := middleware.GetSession(r)
		if session.SellerID == 0 {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		seller, err := h.sellers.GetByID(r.Context(), session.SellerID)
		if err != nil || seller == nil || !seller.IsActive {
			session.SellerID = 0
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next(w, r)
	}
}

// sendApprovalEmail sends an approval notification to the site admin.
// SMTP failure is logged and does not block account creation.
func (h *AuthHandler) sendApprovalEmail(approvalToken string) error {
	approvalURL := fmt.Sprintf("%s/admin/sellers/approve?token=%s", h.config.BaseURL, approvalToken)

	body := fmt.Sprintf("A new seller has registered on Clay.nz and is awaiting approval.\n\n"+
		"Click the link below to approve their account:\n%s\n\n"+
		"If you did not expect this registration, ignore this email.\n", approvalURL)

	if h.config.SMTPHost == "" {
		log.Printf("SMTP not configured. Approval email would be:\nTo: %s\nSubject: New Seller Registration\n%s",
			h.config.OrderEmail, body)
		return nil
	}

	from := h.config.SMTPFrom
	to := h.config.OrderEmail
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: New Seller Registration — Approval Required\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, body)

	addr := fmt.Sprintf("%s:%s", h.config.SMTPHost, h.config.SMTPPort)
	auth := smtp.PlainAuth("", h.config.SMTPUser, h.config.SMTPPass, h.config.SMTPHost)

	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

// DashboardProducts handles GET /dashboard/products — lists the seller's own products.
func (h *AuthHandler) DashboardProducts(w http.ResponseWriter, r *http.Request) {
	h.requireSeller(h.dashboardProducts)(w, r)
}

func (h *AuthHandler) dashboardProducts(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)

	seller, err := h.sellers.GetByID(r.Context(), session.SellerID)
	if err != nil || seller == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	products, err := h.products.ListBySeller(r.Context(), session.SellerID)
	if err != nil {
		log.Printf("DashboardProducts: error listing products: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"PageTitle": "My Products",
		"Seller":    seller,
		"Products":  products,
		"Flash":     session.Flash,
	}
	session.Flash = ""
	h.render(w, "dashboard_products.html", data)
}

// DashboardNewProduct handles GET /dashboard/products/new — renders the new product form.
func (h *AuthHandler) DashboardNewProduct(w http.ResponseWriter, r *http.Request) {
	h.requireSeller(h.dashboardNewProduct)(w, r)
}

func (h *AuthHandler) dashboardNewProduct(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	data := map[string]interface{}{
		"PageTitle": "New Product",
		"Product":   &models.Product{},
		"IsNew":     true,
		"Flash":     session.Flash,
	}
	session.Flash = ""
	h.render(w, "dashboard_product_form.html", data)
}

// DashboardCreateProduct handles POST /dashboard/products/create — creates a product owned by the seller.
func (h *AuthHandler) DashboardCreateProduct(w http.ResponseWriter, r *http.Request) {
	h.requireSeller(h.dashboardCreateProduct)(w, r)
}

func (h *AuthHandler) dashboardCreateProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := middleware.GetSession(r)

	price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
	product := &models.Product{
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		Price:       price,
		IsSold:      r.FormValue("is_sold") == "on",
	}

	if err := h.products.Create(product, session.SellerID); err != nil {
		log.Printf("DashboardCreateProduct: error creating product: %v", err)
		http.Error(w, "Error creating product", http.StatusInternalServerError)
		return
	}

	// Handle image uploads using the shared admin upload logic.
	h.handleImageUploads(r, product.ID)

	http.Redirect(w, r, "/dashboard/products", http.StatusSeeOther)
}

// DashboardEditProduct handles GET /dashboard/products/{id}/edit — renders edit form for a seller's product.
func (h *AuthHandler) DashboardEditProduct(w http.ResponseWriter, r *http.Request) {
	h.requireSeller(h.dashboardEditProduct)(w, r)
}

func (h *AuthHandler) dashboardEditProduct(w http.ResponseWriter, r *http.Request) {
	// Path: /dashboard/products/{id}/edit
	path := r.URL.Path // e.g. /dashboard/products/42/edit
	path = strings.TrimPrefix(path, "/dashboard/products/")
	path = strings.TrimSuffix(path, "/edit")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	session := middleware.GetSession(r)
	product, err := h.products.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if product.SellerID != session.SellerID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	data := map[string]interface{}{
		"PageTitle": "Edit Product",
		"Product":   product,
		"IsNew":     false,
		"Flash":     session.Flash,
	}
	session.Flash = ""
	h.render(w, "dashboard_product_form.html", data)
}

// DashboardUpdateProduct handles POST /dashboard/products/update — updates a seller's product.
func (h *AuthHandler) DashboardUpdateProduct(w http.ResponseWriter, r *http.Request) {
	h.requireSeller(h.dashboardUpdateProduct)(w, r)
}

func (h *AuthHandler) dashboardUpdateProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.ParseMultipartForm(32 << 20)

	session := middleware.GetSession(r)

	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	existing, err := h.products.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if existing.SellerID != session.SellerID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
	product := &models.Product{
		ID:          id,
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		Price:       price,
		IsSold:      r.FormValue("is_sold") == "on",
	}

	if err := h.products.Update(product); err != nil {
		log.Printf("DashboardUpdateProduct: error updating product: %v", err)
		http.Error(w, "Error updating product", http.StatusInternalServerError)
		return
	}

	h.handleImageUploads(r, product.ID)

	http.Redirect(w, r, "/dashboard/products", http.StatusSeeOther)
}

// DashboardDeleteProduct handles POST /dashboard/products/delete — deletes a seller's product.
func (h *AuthHandler) DashboardDeleteProduct(w http.ResponseWriter, r *http.Request) {
	h.requireSeller(h.dashboardDeleteProduct)(w, r)
}

func (h *AuthHandler) dashboardDeleteProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := middleware.GetSession(r)

	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	existing, err := h.products.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if existing.SellerID != session.SellerID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Remove image files from disk.
	images, _ := h.products.GetImages(id)
	for _, img := range images {
		os.Remove(filepath.Join(h.uploadDir, img.Filename))
		if img.ThumbnailFn != "" {
			os.Remove(filepath.Join(h.thumbDir, img.ThumbnailFn))
		}
	}

	if err := h.products.Delete(id); err != nil {
		log.Printf("DashboardDeleteProduct: error deleting product: %v", err)
		http.Error(w, "Error deleting product", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/dashboard/products", http.StatusSeeOther)
}

// DashboardToggleSold handles POST /dashboard/products/toggle-sold — toggles sold status for a seller's product.
func (h *AuthHandler) DashboardToggleSold(w http.ResponseWriter, r *http.Request) {
	h.requireSeller(h.dashboardToggleSold)(w, r)
}

func (h *AuthHandler) dashboardToggleSold(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := middleware.GetSession(r)

	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	product, err := h.products.GetByID(id)
	if err != nil {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}
	if product.SellerID != session.SellerID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	product.IsSold = !product.IsSold
	h.products.Update(product)

	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/dashboard/products"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

// handleImageUploads saves uploaded image files and creates image records for a product.
// Mirrors AdminHandler.handleImageUploads using the auth handler's upload/thumb dirs.
func (h *AuthHandler) handleImageUploads(r *http.Request, productID int64) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		log.Printf("handleImageUploads: error parsing form: %v", err)
		return
	}
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return
	}

	files := r.MultipartForm.File["images"]
	if len(files) == 0 {
		return
	}

	currentCount, _ := h.products.CountImages(productID)
	maxNew := 5 - currentCount
	if maxNew <= 0 {
		return
	}

	for i, fh := range files {
		if i >= maxNew {
			break
		}

		file, err := fh.Open()
		if err != nil {
			continue
		}

		ct := fh.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "image/") {
			file.Close()
			continue
		}

		ext := filepath.Ext(fh.Filename)
		if ext == "" {
			ext = ".jpg"
		}
		filename := fmt.Sprintf("%d_%d_%d%s", productID, time.Now().UnixNano(), i, ext)
		thumbFilename := fmt.Sprintf("thumb_%s", filename)

		dst, err := os.Create(filepath.Join(h.uploadDir, filename))
		if err != nil {
			log.Printf("handleImageUploads: error saving upload: %v", err)
			file.Close()
			continue
		}
		if _, err := io.Copy(dst, file); err != nil {
			dst.Close()
			file.Close()
			continue
		}
		dst.Close()
		file.Close()

		generateThumbnail(
			filepath.Join(h.uploadDir, filename),
			filepath.Join(h.thumbDir, thumbFilename),
			400,
		)

		img := &models.Image{
			ProductID:   productID,
			Filename:    filename,
			ThumbnailFn: thumbFilename,
			SortOrder:   currentCount + i,
		}
		h.products.AddImage(img)
	}
}

// renderLoginError re-renders the login form with an error message.
func (h *AuthHandler) renderLoginError(w http.ResponseWriter, r *http.Request, errMsg string) {
	session := middleware.GetSession(r)
	data := map[string]interface{}{
		"PageTitle": "Seller Login",
		"Error":     errMsg,
		"Flash":     session.Flash,
	}
	session.Flash = ""
	h.render(w, "login.html", data)
}

// renderRegisterError re-renders the registration form with an error message.
func (h *AuthHandler) renderRegisterError(w http.ResponseWriter, r *http.Request, errMsg string) {
	session := middleware.GetSession(r)
	data := map[string]interface{}{
		"PageTitle": "Seller Registration",
		"Error":     errMsg,
		"Flash":     session.Flash,
	}
	session.Flash = ""
	h.render(w, "register.html", data)
}

func (h *AuthHandler) render(w http.ResponseWriter, name string, data interface{}) {
	err := h.templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Printf("Template error (%s): %v", name, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// generateThumbnail resizes src image to maxWidth and saves to dst.
// Shared by AdminHandler and AuthHandler image upload logic.
func generateThumbnail(src, dst string, maxWidth int) {
	img, err := imaging.Open(src, imaging.AutoOrientation(true))
	if err != nil {
		log.Printf("generateThumbnail: error opening image: %v", err)
		return
	}
	thumb := imaging.Resize(img, maxWidth, 0, imaging.Lanczos)
	if err := imaging.Save(thumb, dst, imaging.JPEGQuality(85)); err != nil {
		log.Printf("generateThumbnail: error saving thumbnail: %v", err)
	}
}
