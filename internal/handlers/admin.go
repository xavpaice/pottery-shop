package handlers

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"

	"pottery-shop/internal/models"
)

type AdminHandler struct {
	Store      *models.ProductStore
	Templates  *template.Template
	UploadDir  string
	ThumbDir   string
}

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	products, err := h.Store.ListAll()
	if err != nil {
		log.Printf("Error listing products: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}

	data := map[string]interface{}{
		"Products": products,
	}
	h.render(w, "admin_dashboard.html", data)
}

func (h *AdminHandler) NewProduct(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Product": &models.Product{},
		"IsNew":   true,
	}
	h.render(w, "admin_product_form.html", data)
}

func (h *AdminHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}

	price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
	product := &models.Product{
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		Price:       price,
		IsSold:      r.FormValue("is_sold") == "on",
	}

	if err := h.Store.Create(product); err != nil {
		log.Printf("Error creating product: %v", err)
		http.Error(w, "Error creating product", 500)
		return
	}

	// Handle image uploads
	h.handleImageUploads(r, product.ID)

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *AdminHandler) EditProduct(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/admin/products/"):]
	idStr = strings.TrimSuffix(idStr, "/edit")
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

	data := map[string]interface{}{
		"Product": product,
		"IsNew":   false,
	}
	h.render(w, "admin_product_form.html", data)
}

func (h *AdminHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}

	// Parse multipart first so both form values and files are available
	r.ParseMultipartForm(32 << 20)

	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid product ID", 400)
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

	if err := h.Store.Update(product); err != nil {
		log.Printf("Error updating product: %v", err)
		http.Error(w, "Error updating product", 500)
		return
	}

	// Handle new image uploads
	h.handleImageUploads(r, product.ID)

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *AdminHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}

	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid product ID", 400)
		return
	}

	// Delete associated images from disk
	images, _ := h.Store.GetImages(id)
	for _, img := range images {
		os.Remove(filepath.Join(h.UploadDir, img.Filename))
		if img.ThumbnailFn != "" {
			os.Remove(filepath.Join(h.ThumbDir, img.ThumbnailFn))
		}
	}

	if err := h.Store.Delete(id); err != nil {
		log.Printf("Error deleting product: %v", err)
		http.Error(w, "Error deleting product", 500)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *AdminHandler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}

	imageID, err := strconv.ParseInt(r.FormValue("image_id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid image ID", 400)
		return
	}

	img, err := h.Store.DeleteImage(imageID)
	if err != nil {
		log.Printf("Error deleting image: %v", err)
		http.Error(w, "Error deleting image", 500)
		return
	}

	os.Remove(filepath.Join(h.UploadDir, img.Filename))
	if img.ThumbnailFn != "" {
		os.Remove(filepath.Join(h.ThumbDir, img.ThumbnailFn))
	}

	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/admin"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

func (h *AdminHandler) ToggleSold(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}

	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid product ID", 400)
		return
	}

	product, err := h.Store.GetByID(id)
	if err != nil {
		http.Error(w, "Product not found", 404)
		return
	}

	product.IsSold = !product.IsSold
	h.Store.Update(product)

	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/admin"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

func (h *AdminHandler) handleImageUploads(r *http.Request, productID int64) {
	err := r.ParseMultipartForm(32 << 20) // 32MB max
	if err != nil {
		log.Printf("Error parsing multipart form: %v", err)
		return
	}

	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return
	}

	files := r.MultipartForm.File["images"]
	if len(files) == 0 {
		return
	}

	currentCount, _ := h.Store.CountImages(productID)
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

		// Validate content type
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

		// Save original
		dst, err := os.Create(filepath.Join(h.UploadDir, filename))
		if err != nil {
			log.Printf("Error saving upload: %v", err)
			continue
		}

		if _, err := io.Copy(dst, file); err != nil {
			dst.Close()
			file.Close()
			continue
		}
		dst.Close()
		file.Close()

		// Generate thumbnail
		h.generateThumbnail(
			filepath.Join(h.UploadDir, filename),
			filepath.Join(h.ThumbDir, thumbFilename),
			400,
		)

		img := &models.Image{
			ProductID:   productID,
			Filename:    filename,
			ThumbnailFn: thumbFilename,
			SortOrder:   currentCount + i,
		}
		h.Store.AddImage(img)
	}
}

func (h *AdminHandler) generateThumbnail(src, dst string, maxWidth int) {
	// imaging.Open auto-applies EXIF orientation
	img, err := imaging.Open(src, imaging.AutoOrientation(true))
	if err != nil {
		log.Printf("Error opening image for thumbnail: %v", err)
		return
	}

	thumb := imaging.Resize(img, maxWidth, 0, imaging.Lanczos)

	if err := imaging.Save(thumb, dst, imaging.JPEGQuality(85)); err != nil {
		log.Printf("Error saving thumbnail: %v", err)
	}
}

func (h *AdminHandler) render(w http.ResponseWriter, name string, data interface{}) {
	err := h.Templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Printf("Template error (%s): %v", name, err)
		http.Error(w, "Internal server error", 500)
	}
}
