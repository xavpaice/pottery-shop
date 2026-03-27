package handlers

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/smtp"
	"strconv"

	"pottery-shop/internal/middleware"
	"pottery-shop/internal/models"
)

type PublicHandler struct {
	Store     *models.ProductStore
	Templates *template.Template
	Session   *middleware.SessionManager
	Config    *Config
}

type Config struct {
	SMTPHost   string
	SMTPPort   string
	SMTPUser   string
	SMTPPass   string
	SMTPFrom   string
	OrderEmail string
	BaseURL    string
}

func (h *PublicHandler) Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	products, err := h.Store.ListAvailable()
	if err != nil {
		log.Printf("Error listing products: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}

	session := middleware.GetSession(r)
	cart := models.CartFromJSON(session.CartJSON)

	data := map[string]interface{}{
		"Products":  products,
		"CartCount": cart.Count(),
		"Flash":     session.Flash,
	}
	session.Flash = ""

	h.render(w, "home.html", data)
}

func (h *PublicHandler) Gallery(w http.ResponseWriter, r *http.Request) {
	products, err := h.Store.ListSold()
	if err != nil {
		log.Printf("Error listing sold products: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}

	session := middleware.GetSession(r)
	cart := models.CartFromJSON(session.CartJSON)

	data := map[string]interface{}{
		"Products":  products,
		"CartCount": cart.Count(),
		"Flash":     session.Flash,
	}
	session.Flash = ""

	h.render(w, "gallery.html", data)
}

func (h *PublicHandler) ProductDetail(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/product/"):]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	product, err := h.Store.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	session := middleware.GetSession(r)
	cart := models.CartFromJSON(session.CartJSON)

	inCart := false
	for _, item := range cart.Items {
		if item.ProductID == product.ID {
			inCart = true
			break
		}
	}

	data := map[string]interface{}{
		"Product":   product,
		"CartCount": cart.Count(),
		"InCart":    inCart,
		"Flash":    session.Flash,
	}
	session.Flash = ""

	h.render(w, "product.html", data)
}

func (h *PublicHandler) AddToCart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}

	productID, err := strconv.ParseInt(r.FormValue("product_id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid product", 400)
		return
	}

	product, err := h.Store.GetByID(productID)
	if err != nil || product.IsSold {
		http.Error(w, "Product not available", 400)
		return
	}

	session := middleware.GetSession(r)
	cart := models.CartFromJSON(session.CartJSON)

	thumb := ""
	if len(product.Images) > 0 {
		thumb = product.Images[0].ThumbnailFn
	}

	cart.Add(models.CartItem{
		ProductID: product.ID,
		Title:     product.Title,
		Price:     product.Price,
		Thumbnail: thumb,
	})

	cartJSON, _ := cart.Marshal()
	session.CartJSON = cartJSON
	session.Flash = fmt.Sprintf("'%s' added to cart", product.Title)

	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

func (h *PublicHandler) RemoveFromCart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}

	productID, err := strconv.ParseInt(r.FormValue("product_id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid product", 400)
		return
	}

	session := middleware.GetSession(r)
	cart := models.CartFromJSON(session.CartJSON)
	cart.Remove(productID)

	cartJSON, _ := cart.Marshal()
	session.CartJSON = cartJSON

	http.Redirect(w, r, "/cart", http.StatusSeeOther)
}

func (h *PublicHandler) ViewCart(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	cart := models.CartFromJSON(session.CartJSON)

	data := map[string]interface{}{
		"Cart":      cart,
		"CartCount": cart.Count(),
		"Total":     cart.Total(),
		"Flash":     session.Flash,
	}
	session.Flash = ""

	h.render(w, "cart.html", data)
}

func (h *PublicHandler) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}

	session := middleware.GetSession(r)
	cart := models.CartFromJSON(session.CartJSON)

	if cart.Count() == 0 {
		session.Flash = "Your cart is empty"
		http.Redirect(w, r, "/cart", http.StatusSeeOther)
		return
	}

	buyerName := r.FormValue("name")
	buyerEmail := r.FormValue("email")
	message := r.FormValue("message")

	if buyerName == "" || buyerEmail == "" {
		session.Flash = "Please provide your name and email"
		http.Redirect(w, r, "/cart", http.StatusSeeOther)
		return
	}

	// Build email body
	body := fmt.Sprintf("New order from %s (%s)\n\n", buyerName, buyerEmail)
	if message != "" {
		body += fmt.Sprintf("Message: %s\n\n", message)
	}
	body += "Items:\n"
	body += "------\n"
	for _, item := range cart.Items {
		body += fmt.Sprintf("- %s — $%.2f\n  %s/product/%d\n", item.Title, item.Price, h.Config.BaseURL, item.ProductID)
	}
	body += fmt.Sprintf("\nTotal: $%.2f\n", cart.Total())

	err := h.sendEmail(
		fmt.Sprintf("New Pottery Order from %s", buyerName),
		body,
	)
	if err != nil {
		log.Printf("Error sending order email: %v", err)
		session.Flash = "Order received but email notification failed. We'll follow up."
	} else {
		session.Flash = "Order placed! We'll be in touch soon."
	}

	// Clear the cart
	session.CartJSON = ""

	http.Redirect(w, r, "/order-confirmed", http.StatusSeeOther)
}

func (h *PublicHandler) OrderConfirmed(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	data := map[string]interface{}{
		"CartCount": 0,
		"Flash":     session.Flash,
	}
	session.Flash = ""
	h.render(w, "order_confirmed.html", data)
}

func (h *PublicHandler) sendEmail(subject, body string) error {
	if h.Config.SMTPHost == "" {
		log.Printf("SMTP not configured. Email would be:\nTo: %s\nSubject: %s\n%s", h.Config.OrderEmail, subject, body)
		return nil
	}

	from := h.Config.SMTPFrom
	to := h.Config.OrderEmail
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	addr := fmt.Sprintf("%s:%s", h.Config.SMTPHost, h.Config.SMTPPort)
	auth := smtp.PlainAuth("", h.Config.SMTPUser, h.Config.SMTPPass, h.Config.SMTPHost)

	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

func (h *PublicHandler) render(w http.ResponseWriter, name string, data interface{}) {
	err := h.Templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Printf("Template error (%s): %v", name, err)
		http.Error(w, "Internal server error", 500)
	}
}
