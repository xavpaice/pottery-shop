package models

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Seller represents a registered seller account.
type Seller struct {
	ID            int64     `db:"id"`
	Email         string    `db:"email"`
	PasswordHash  string    `db:"password_hash"`
	Name          string    `db:"name"`
	Bio           string    `db:"bio"`
	OrderEmail    string    `db:"order_email"`
	IsActive      bool      `db:"is_active"`
	IsAdmin       bool      `db:"is_admin"`
	ApprovalToken string    `db:"approval_token"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

// SellerStore provides data access methods for seller accounts.
type SellerStore struct {
	pool *pgxpool.Pool
}

// NewSellerStore creates a new SellerStore backed by the given connection pool.
func NewSellerStore(pool *pgxpool.Pool) *SellerStore {
	return &SellerStore{pool: pool}
}

// Create inserts a new seller with a hashed password and a random approval token.
// The seller is created with is_active=false and is_admin=false.
func (s *SellerStore) Create(ctx context.Context, email, plaintextPassword, name string) (*Seller, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return nil, err
	}

	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	seller := &Seller{}
	err = s.pool.QueryRow(ctx,
		`INSERT INTO sellers (email, password_hash, name, approval_token)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, email, password_hash, name, bio, order_email, is_active, is_admin, approval_token, created_at, updated_at`,
		email, string(hash), name, token,
	).Scan(
		&seller.ID, &seller.Email, &seller.PasswordHash, &seller.Name,
		&seller.Bio, &seller.OrderEmail, &seller.IsActive, &seller.IsAdmin,
		&seller.ApprovalToken, &seller.CreatedAt, &seller.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return seller, nil
}

// CreateAdmin inserts a new seller with is_active=true and is_admin=true.
// Used for bootstrapping the first administrator at startup.
func (s *SellerStore) CreateAdmin(ctx context.Context, email, plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO sellers (email, password_hash, name, is_active, is_admin, approval_token)
		 VALUES ($1, $2, $3, true, true, '')`,
		email, string(hash), email,
	)
	return err
}

// GetByEmail returns the seller with the given email, or nil if not found.
func (s *SellerStore) GetByEmail(ctx context.Context, email string) (*Seller, error) {
	seller := &Seller{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, name, bio, order_email, is_active, is_admin, approval_token, created_at, updated_at
		 FROM sellers WHERE email=$1`,
		email,
	).Scan(
		&seller.ID, &seller.Email, &seller.PasswordHash, &seller.Name,
		&seller.Bio, &seller.OrderEmail, &seller.IsActive, &seller.IsAdmin,
		&seller.ApprovalToken, &seller.CreatedAt, &seller.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return seller, nil
}

// GetByID returns the seller with the given ID, or nil if not found.
func (s *SellerStore) GetByID(ctx context.Context, id int64) (*Seller, error) {
	seller := &Seller{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, name, bio, order_email, is_active, is_admin, approval_token, created_at, updated_at
		 FROM sellers WHERE id=$1`,
		id,
	).Scan(
		&seller.ID, &seller.Email, &seller.PasswordHash, &seller.Name,
		&seller.Bio, &seller.OrderEmail, &seller.IsActive, &seller.IsAdmin,
		&seller.ApprovalToken, &seller.CreatedAt, &seller.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return seller, nil
}

// GetByApprovalToken returns the seller with the given approval token, or nil if not found.
func (s *SellerStore) GetByApprovalToken(ctx context.Context, token string) (*Seller, error) {
	seller := &Seller{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, name, bio, order_email, is_active, is_admin, approval_token, created_at, updated_at
		 FROM sellers WHERE approval_token=$1`,
		token,
	).Scan(
		&seller.ID, &seller.Email, &seller.PasswordHash, &seller.Name,
		&seller.Bio, &seller.OrderEmail, &seller.IsActive, &seller.IsAdmin,
		&seller.ApprovalToken, &seller.CreatedAt, &seller.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return seller, nil
}

// Approve sets is_active=true and clears the approval_token for the seller
// identified by the given token.
func (s *SellerStore) Approve(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sellers SET is_active=true, approval_token='', updated_at=now() WHERE approval_token=$1`,
		token,
	)
	return err
}

// CheckPassword returns true if the plaintext password matches the seller's stored hash.
func (s *SellerStore) CheckPassword(seller *Seller, plaintext string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(seller.PasswordHash), []byte(plaintext))
	return err == nil
}

// UpdateProfile updates the name, bio, and order email for the seller with the given ID.
func (s *SellerStore) UpdateProfile(ctx context.Context, id int64, name, bio, orderEmail string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sellers SET name=$1, bio=$2, order_email=$3, updated_at=now() WHERE id=$4`,
		name, bio, orderEmail, id,
	)
	return err
}

// ListAll returns all sellers ordered by creation date descending. Used by admin.
func (s *SellerStore) ListAll(ctx context.Context) ([]Seller, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, email, password_hash, name, bio, order_email, is_active, is_admin, approval_token, created_at, updated_at
		 FROM sellers ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sellers []Seller
	for rows.Next() {
		var sel Seller
		if err := rows.Scan(
			&sel.ID, &sel.Email, &sel.PasswordHash, &sel.Name,
			&sel.Bio, &sel.OrderEmail, &sel.IsActive, &sel.IsAdmin,
			&sel.ApprovalToken, &sel.CreatedAt, &sel.UpdatedAt,
		); err != nil {
			return nil, err
		}
		sellers = append(sellers, sel)
	}
	return sellers, rows.Err()
}

// SetActive enables or disables a seller account. Used by admin.
func (s *SellerStore) SetActive(ctx context.Context, id int64, active bool) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sellers SET is_active=$1, updated_at=now() WHERE id=$2`,
		active, id,
	)
	return err
}

// AdminExists returns true if at least one seller with is_admin=true exists.
func (s *SellerStore) AdminExists(ctx context.Context) (bool, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM sellers WHERE is_admin=true`,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// generateToken creates a cryptographically random 32-byte hex string.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
