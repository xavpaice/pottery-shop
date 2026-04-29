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
	Sellers   *models.SellerStore
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

	products, err := h.Store.ListAvailableWithSeller(r.Context())
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
	ctx := r.Context()

	var products []models.Product
	var err error

	// Optional seller filter: ?seller={id}
	if sellerStr := r.URL.Query().Get("seller"); sellerStr != "" {
		if sellerID, parseErr := strconv.ParseInt(sellerStr, 10, 64); parseErr == nil {
			products, err = h.Store.ListBySeller(ctx, sellerID)
			// Filter to only sold items when filtering by seller
			if err == nil {
				var sold []models.Product
				for _, p := range products {
					if p.IsSold {
						sold = append(sold, p)
					}
				}
				products = sold
			}
		}
	}

	// No (valid) seller filter — list all sold with seller attribution
	if products == nil && err == nil {
		products, err = h.Store.ListSoldWithSeller(ctx)
	}

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

	product, err := h.Store.GetByIDWithSeller(r.Context(), id)
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

func (h *PublicHandler) SellerProfile(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ctx := r.Context()
	seller, err := h.Sellers.GetByID(ctx, id)
	if err != nil || seller == nil {
		http.NotFound(w, r)
		return
	}

	allProducts, err := h.Store.ListBySeller(ctx, id)
	if err != nil {
		log.Printf("Error listing products for seller %d: %v", id, err)
		http.Error(w, "Internal server error", 500)
		return
	}

	var available, pastWork []models.Product
	for _, p := range allProducts {
		if p.IsSold {
			pastWork = append(pastWork, p)
		} else {
			available = append(available, p)
		}
	}

	session := middleware.GetSession(r)
	cart := models.CartFromJSON(session.CartJSON)

	data := map[string]interface{}{
		"Seller":    seller,
		"Available": available,
		"PastWork":  pastWork,
		"CartCount": cart.Count(),
		"Flash":     session.Flash,
	}
	session.Flash = ""

	h.render(w, "seller_profile.html", data)
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

	log.Printf("AddToCart: product added: id=%d title=%q price=%.2f cart_count=%d", product.ID, product.Title, product.Price, cart.Count())

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

	step := 1
	if s := r.URL.Query().Get("step"); s == "2" {
		step = 2
	} else if s == "3" {
		step = 3
	}

	data := map[string]interface{}{
		"Cart":      cart,
		"CartCount": cart.Count(),
		"Total":     cart.Total(),
		"Flash":     session.Flash,
		"Step":      step,
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

	// Determine the order email recipient: use the first cart item's seller, fall back to global ORDER_EMAIL.
	toEmail := h.Config.OrderEmail
	if len(cart.Items) > 0 {
		product, perr := h.Store.GetByID(cart.Items[0].ProductID)
		if perr == nil && product.SellerID != 0 {
			seller, serr := h.Sellers.GetByID(r.Context(), product.SellerID)
			if serr == nil && seller != nil && seller.OrderEmail != "" {
				toEmail = seller.OrderEmail
			}
		}
	}

	for _, item := range cart.Items {
		body += fmt.Sprintf("- %s — $%.2f\n  %s/product/%d\n", item.Title, item.Price, h.Config.BaseURL, item.ProductID)
	}
	body += fmt.Sprintf("\nTotal: $%.2f\n", cart.Total())

	err := h.sendEmailTo(
		toEmail,
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

// sendEmailTo sends an email to the given recipient address.
func (h *PublicHandler) sendEmailTo(to, subject, body string) error {
	if h.Config.SMTPHost == "" {
		log.Printf("SMTP not configured. Email would be:\nTo: %s\nSubject: %s\n%s", to, subject, body)
		return nil
	}

	from := h.Config.SMTPFrom
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	addr := fmt.Sprintf("%s:%s", h.Config.SMTPHost, h.Config.SMTPPort)
	auth := smtp.PlainAuth("", h.Config.SMTPUser, h.Config.SMTPPass, h.Config.SMTPHost)

	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

func (h *PublicHandler) SellersList(w http.ResponseWriter, r *http.Request) {
	sellers, err := h.Sellers.ListActive(r.Context())
	if err != nil {
		log.Printf("Error listing sellers: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}

	session := middleware.GetSession(r)
	cart := models.CartFromJSON(session.CartJSON)

	data := map[string]interface{}{
		"Sellers":   sellers,
		"CartCount": cart.Count(),
		"Flash":     session.Flash,
		"PageTitle": "Makers",
	}
	session.Flash = ""

	h.render(w, "sellers.html", data)
}

func (h *PublicHandler) render(w http.ResponseWriter, name string, data interface{}) {
	err := h.Templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Printf("Template error (%s): %v", name, err)
		http.Error(w, "Internal server error", 500)
	}
}
