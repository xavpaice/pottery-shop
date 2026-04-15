package handlers

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/smtp"
	"strings"

	"pottery-shop/internal/middleware"
	"pottery-shop/internal/models"
)

// AuthHandler handles seller authentication: login, register, logout, dashboard, and approval.
type AuthHandler struct {
	sellers   *models.SellerStore
	sessions  *middleware.SessionManager
	templates *template.Template
	config    *Config
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(sellers *models.SellerStore, sessions *middleware.SessionManager, templates *template.Template, config *Config) *AuthHandler {
	return &AuthHandler{
		sellers:   sellers,
		sessions:  sessions,
		templates: templates,
		config:    config,
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
		h.renderLoginError(w, r, "Invalid email or password")
		return
	}
	if !seller.IsActive {
		h.renderLoginError(w, r, "Your account is pending approval. You will be notified when it is approved.")
		return
	}

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
		log.Printf("Register: error creating seller: %v", err)
		// Check for duplicate email (unique constraint violation)
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			h.renderRegisterError(w, r, "An account with that email already exists")
			return
		}
		http.Error(w, "Registration failed, please try again", http.StatusInternalServerError)
		return
	}

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
		"PageTitle": "Seller Dashboard",
		"Seller":    seller,
		"Flash":     session.Flash,
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html><html><body><p>Seller <strong>%s</strong> approved. They can now log in.</p></body></html>`,
		template.HTMLEscapeString(seller.Name))
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
