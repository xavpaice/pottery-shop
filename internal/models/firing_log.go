package models

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FiringLog represents a single kiln firing session recorded by a seller.
type FiringLog struct {
	ID         int64           `db:"id"`
	SellerID   int64           `db:"seller_id"`
	Title      string          `db:"title"`
	FiringDate *string         `db:"firing_date"` // nullable DATE as *string for simplicity
	ClayBody   string          `db:"clay_body"`
	GlazeNotes string          `db:"glaze_notes"`
	Outcome    string          `db:"outcome"`
	Notes      string          `db:"notes"`
	CreatedAt  time.Time       `db:"created_at"`
	UpdatedAt  time.Time       `db:"updated_at"`
	Readings   []FiringReading // populated by GetByID, not a DB column
}

// FiringReading represents a single temperature/settings reading during a firing.
type FiringReading struct {
	ID             int64     `db:"id"`
	FiringLogID    int64     `db:"firing_log_id"`
	ElapsedMinutes int       `db:"elapsed_minutes"`
	Temperature    float64   `db:"temperature"`
	GasSetting     string    `db:"gas_setting"`
	FlueSetting    string    `db:"flue_setting"`
	Notes          string    `db:"notes"`
	CreatedAt      time.Time `db:"created_at"`
}

// FiringLogStore provides data access methods for firing logs and their readings.
type FiringLogStore struct {
	pool *pgxpool.Pool
}

// NewFiringLogStore creates a new FiringLogStore backed by the given connection pool.
func NewFiringLogStore(pool *pgxpool.Pool) *FiringLogStore {
	return &FiringLogStore{pool: pool}
}

// Create inserts a new firing log owned by the given seller.
func (s *FiringLogStore) Create(ctx context.Context, sellerID int64, title, clayBody, glazeNotes, outcome, notes string, firingDate *string) (*FiringLog, error) {
	log := &FiringLog{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO firing_logs (seller_id, title, clay_body, glaze_notes, outcome, notes, firing_date)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, seller_id, title, firing_date::text, clay_body, glaze_notes, outcome, notes, created_at, updated_at`,
		sellerID, title, clayBody, glazeNotes, outcome, notes, firingDate,
	).Scan(
		&log.ID, &log.SellerID, &log.Title, &log.FiringDate,
		&log.ClayBody, &log.GlazeNotes, &log.Outcome, &log.Notes,
		&log.CreatedAt, &log.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return log, nil
}

// GetByID returns the firing log with the given ID, enforcing ownership via sellerID.
// Returns nil if the log does not exist or belongs to a different seller.
// Also populates the Readings field.
func (s *FiringLogStore) GetByID(ctx context.Context, id, sellerID int64) (*FiringLog, error) {
	fl := &FiringLog{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, seller_id, title, firing_date::text, clay_body, glaze_notes, outcome, notes, created_at, updated_at
		 FROM firing_logs WHERE id=$1 AND seller_id=$2`,
		id, sellerID,
	).Scan(
		&fl.ID, &fl.SellerID, &fl.Title, &fl.FiringDate,
		&fl.ClayBody, &fl.GlazeNotes, &fl.Outcome, &fl.Notes,
		&fl.CreatedAt, &fl.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Populate readings.
	readings, err := s.GetReadingsForAPI(ctx, id, sellerID)
	if err != nil {
		return nil, err
	}
	fl.Readings = readings
	return fl, nil
}

// ListBySeller returns all firing logs for the given seller, ordered by firing_date DESC NULLS LAST, created_at DESC.
// Readings are not populated on list results.
func (s *FiringLogStore) ListBySeller(ctx context.Context, sellerID int64) ([]FiringLog, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, seller_id, title, firing_date::text, clay_body, glaze_notes, outcome, notes, created_at, updated_at
		 FROM firing_logs WHERE seller_id=$1
		 ORDER BY firing_date DESC NULLS LAST, created_at DESC`,
		sellerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []FiringLog
	for rows.Next() {
		var fl FiringLog
		if err := rows.Scan(
			&fl.ID, &fl.SellerID, &fl.Title, &fl.FiringDate,
			&fl.ClayBody, &fl.GlazeNotes, &fl.Outcome, &fl.Notes,
			&fl.CreatedAt, &fl.UpdatedAt,
		); err != nil {
			return nil, err
		}
		logs = append(logs, fl)
	}
	return logs, rows.Err()
}

// Update modifies the fields of a firing log, enforcing ownership via sellerID in the WHERE clause.
func (s *FiringLogStore) Update(ctx context.Context, id, sellerID int64, title, clayBody, glazeNotes, outcome, notes string, firingDate *string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE firing_logs SET title=$1, clay_body=$2, glaze_notes=$3, outcome=$4, notes=$5, firing_date=$6, updated_at=now()
		 WHERE id=$7 AND seller_id=$8`,
		title, clayBody, glazeNotes, outcome, notes, firingDate, id, sellerID,
	)
	return err
}

// Delete removes a firing log, enforcing ownership via sellerID.
// CASCADE on firing_readings handles deletion of related readings.
func (s *FiringLogStore) Delete(ctx context.Context, id, sellerID int64) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM firing_logs WHERE id=$1 AND seller_id=$2`,
		id, sellerID,
	)
	return err
}

// SaveReadings replaces all readings for a firing log in a single transaction.
// Ownership is verified first via GetByID. Existing readings are deleted and re-inserted.
func (s *FiringLogStore) SaveReadings(ctx context.Context, firingLogID, sellerID int64, readings []FiringReading) error {
	// Verify ownership.
	fl, err := s.GetByID(ctx, firingLogID, sellerID)
	if err != nil {
		return err
	}
	if fl == nil {
		return pgx.ErrNoRows
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Delete existing readings.
	if _, err := tx.Exec(ctx, `DELETE FROM firing_readings WHERE firing_log_id=$1`, firingLogID); err != nil {
		return err
	}

	// Insert new readings.
	for _, r := range readings {
		if _, err := tx.Exec(ctx,
			`INSERT INTO firing_readings (firing_log_id, elapsed_minutes, temperature, gas_setting, flue_setting, notes)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			firingLogID, r.ElapsedMinutes, r.Temperature, r.GasSetting, r.FlueSetting, r.Notes,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// GetReadingsForAPI returns readings for a firing log ordered by elapsed_minutes, with ownership enforced.
func (s *FiringLogStore) GetReadingsForAPI(ctx context.Context, id, sellerID int64) ([]FiringReading, error) {
	// Verify the log belongs to this seller (without populating readings to avoid recursion).
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM firing_logs WHERE id=$1 AND seller_id=$2`,
		id, sellerID,
	).Scan(&count)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, firing_log_id, elapsed_minutes, temperature, gas_setting, flue_setting, notes, created_at
		 FROM firing_readings WHERE firing_log_id=$1
		 ORDER BY elapsed_minutes`,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var readings []FiringReading
	for rows.Next() {
		var r FiringReading
		if err := rows.Scan(
			&r.ID, &r.FiringLogID, &r.ElapsedMinutes, &r.Temperature,
			&r.GasSetting, &r.FlueSetting, &r.Notes, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		readings = append(readings, r)
	}
	return readings, rows.Err()
}
