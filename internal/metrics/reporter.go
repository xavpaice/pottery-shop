package metrics

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Reporter struct {
	db       *sql.DB
	pool     *pgxpool.Pool
	endpoint string
	interval time.Duration
}

func NewReporter(db *sql.DB, pool *pgxpool.Pool, sdkServiceName string, interval time.Duration) *Reporter {
	return &Reporter{
		db:       db,
		pool:     pool,
		endpoint: fmt.Sprintf("http://%s:3000/api/v1/app/custom-metrics", sdkServiceName),
		interval: interval,
	}
}

func (r *Reporter) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Send once on startup after a short delay for the SDK to be ready
	time.Sleep(10 * time.Second)
	r.collect(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.collect(ctx)
		}
	}
}

func (r *Reporter) collect(ctx context.Context) {
	data := make(map[string]int)

	if n, err := r.countQuery(ctx, "SELECT COUNT(*) FROM products"); err == nil {
		data["numProducts"] = n
	} else {
		log.Printf("metrics: count products: %v", err)
	}

	if n, err := r.countQuery(ctx, "SELECT COUNT(*) FROM products WHERE is_sold = true"); err == nil {
		data["numProductsSold"] = n
	} else {
		log.Printf("metrics: count sold products: %v", err)
	}

	if n, err := r.countQueryPool(ctx, "SELECT COUNT(*) FROM sellers"); err == nil {
		data["numSellers"] = n
	} else {
		log.Printf("metrics: count sellers: %v", err)
	}

	if n, err := r.countQueryPool(ctx, "SELECT COUNT(*) FROM firing_logs"); err == nil {
		data["numFiringLogs"] = n
	} else {
		log.Printf("metrics: count firing logs: %v", err)
	}

	if len(data) == 0 {
		return
	}

	body, err := json.Marshal(map[string]any{"data": data})
	if err != nil {
		log.Printf("metrics: marshal: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, r.endpoint, bytes.NewReader(body))
	if err != nil {
		log.Printf("metrics: create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("metrics: send: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("metrics: SDK returned %d", resp.StatusCode)
	}
}

func (r *Reporter) countQuery(ctx context.Context, query string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, query).Scan(&n)
	return n, err
}

func (r *Reporter) countQueryPool(ctx context.Context, query string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, query).Scan(&n)
	return n, err
}
