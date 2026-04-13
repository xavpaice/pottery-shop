package models

import (
	"database/sql"
	"time"
)

type Product struct {
	ID          int64
	Title       string
	Description string
	Price       float64
	IsSold      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Images      []Image
}

type Image struct {
	ID          int64
	ProductID   int64
	Filename    string
	ThumbnailFn string
	SortOrder   int
	CreatedAt   time.Time
}

type ProductStore struct {
	DB *sql.DB
}

func NewProductStore(db *sql.DB) *ProductStore {
	return &ProductStore{DB: db}
}

func (s *ProductStore) Create(p *Product) error {
	err := s.DB.QueryRow(
		`INSERT INTO products (title, description, price, is_sold)
         VALUES ($1, $2, $3, $4) RETURNING id`,
		p.Title, p.Description, p.Price, p.IsSold,
	).Scan(&p.ID)
	return err
}

func (s *ProductStore) Update(p *Product) error {
	_, err := s.DB.Exec(
		`UPDATE products SET title=$1, description=$2, price=$3, is_sold=$4, updated_at=NOW() WHERE id=$5`,
		p.Title, p.Description, p.Price, p.IsSold, p.ID,
	)
	return err
}

func (s *ProductStore) Delete(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM products WHERE id=$1`, id)
	return err
}

func (s *ProductStore) GetByID(id int64) (*Product, error) {
	p := &Product{}
	err := s.DB.QueryRow(
		`SELECT id, title, description, price, is_sold, created_at, updated_at FROM products WHERE id=$1`, id,
	).Scan(&p.ID, &p.Title, &p.Description, &p.Price, &p.IsSold, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.Images, err = s.GetImages(id)
	return p, err
}

func (s *ProductStore) ListAll() ([]Product, error) {
	return s.listProducts(`SELECT id, title, description, price, is_sold, created_at, updated_at FROM products ORDER BY created_at DESC`)
}

func (s *ProductStore) ListAvailable() ([]Product, error) {
	return s.listProducts(`SELECT id, title, description, price, is_sold, created_at, updated_at FROM products WHERE is_sold=false ORDER BY created_at DESC`)
}

func (s *ProductStore) ListSold() ([]Product, error) {
	return s.listProducts(`SELECT id, title, description, price, is_sold, created_at, updated_at FROM products WHERE is_sold=true ORDER BY updated_at DESC`)
}

func (s *ProductStore) listProducts(query string) ([]Product, error) {
	rows, err := s.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.Price, &p.IsSold, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Images, _ = s.GetImages(p.ID)
		products = append(products, p)
	}
	return products, rows.Err()
}

func (s *ProductStore) AddImage(img *Image) error {
	err := s.DB.QueryRow(
		`INSERT INTO images (product_id, filename, thumbnail_fn, sort_order)
         VALUES ($1, $2, $3, $4) RETURNING id`,
		img.ProductID, img.Filename, img.ThumbnailFn, img.SortOrder,
	).Scan(&img.ID)
	return err
}

func (s *ProductStore) GetImages(productID int64) ([]Image, error) {
	rows, err := s.DB.Query(
		`SELECT id, product_id, filename, thumbnail_fn, sort_order, created_at FROM images WHERE product_id=$1 ORDER BY sort_order`,
		productID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []Image
	for rows.Next() {
		var img Image
		if err := rows.Scan(&img.ID, &img.ProductID, &img.Filename, &img.ThumbnailFn, &img.SortOrder, &img.CreatedAt); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

func (s *ProductStore) DeleteImage(id int64) (*Image, error) {
	img := &Image{}
	err := s.DB.QueryRow(`SELECT id, product_id, filename, thumbnail_fn FROM images WHERE id=$1`, id).
		Scan(&img.ID, &img.ProductID, &img.Filename, &img.ThumbnailFn)
	if err != nil {
		return nil, err
	}
	_, err = s.DB.Exec(`DELETE FROM images WHERE id=$1`, id)
	return img, err
}

func (s *ProductStore) CountImages(productID int64) (int, error) {
	var count int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM images WHERE product_id=$1`, productID).Scan(&count)
	return count, err
}
