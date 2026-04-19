package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type AppUpdate struct {
	VersionLabel string `json:"versionLabel"`
	CreatedAt    string `json:"createdAt"`
	ReleaseNotes string `json:"releaseNotes"`
}

type UpdateChecker struct {
	endpoint string
	interval time.Duration

	mu      sync.RWMutex
	updates []AppUpdate
}

func NewUpdateChecker(sdkServiceName string, interval time.Duration) *UpdateChecker {
	return &UpdateChecker{
		endpoint: fmt.Sprintf("http://%s:3000/api/v1/app/updates", sdkServiceName),
		interval: interval,
	}
}

func (u *UpdateChecker) Run(ctx context.Context) {
	// Initial check after short delay
	time.Sleep(15 * time.Second)
	u.check(ctx)

	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			u.check(ctx)
		}
	}
}

func (u *UpdateChecker) Available() []AppUpdate {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.updates
}

func (u *UpdateChecker) check(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.endpoint, nil)
	if err != nil {
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var updates []AppUpdate
	if err := json.NewDecoder(resp.Body).Decode(&updates); err != nil {
		return
	}

	u.mu.Lock()
	u.updates = updates
	u.mu.Unlock()
}
