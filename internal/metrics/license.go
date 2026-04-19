package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type LicenseField struct {
	Name      string      `json:"name"`
	Title     string      `json:"title"`
	Value     interface{} `json:"value"`
	ValueType string      `json:"valueType"`
}

// ValidateLicense checks that the Replicated SDK is reachable and the license is not expired.
// Retries up to 30 times (every 10s) to allow the SDK pod to start.
// Skips validation if REPLICATED_SDK_SERVICE is set to "none".
func ValidateLicense(sdkServiceName string) error {
	if sdkServiceName == "none" {
		log.Println("license: validation skipped (SDK service set to none)")
		return nil
	}

	endpoint := fmt.Sprintf("http://%s:3000/api/v1/license/info", sdkServiceName)

	var lastErr error
	for attempt := 1; attempt <= 30; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			cancel()
			return fmt.Errorf("create request: %w", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			log.Printf("license: SDK not ready (attempt %d/30), retrying in 10s...", attempt)
			time.Sleep(10 * time.Second)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			cancel()
			return fmt.Errorf("SDK returned status %d", resp.StatusCode)
		}

		var info struct {
			LicenseID    string `json:"licenseID"`
			CustomerName string `json:"customerName"`
			LicenseType  string `json:"licenseType"`
			Entitlements map[string]struct {
				Value interface{} `json:"value"`
			} `json:"entitlements"`
		}
		err = json.NewDecoder(resp.Body).Decode(&info)
		resp.Body.Close()
		cancel()
		if err != nil {
			return fmt.Errorf("decode license info: %w", err)
		}

		if info.LicenseID == "" {
			return fmt.Errorf("no license ID returned")
		}

		// Check expiration if present
		if exp, ok := info.Entitlements["expires_at"]; ok {
			if expStr, ok := exp.Value.(string); ok && expStr != "" {
				expTime, err := time.Parse(time.RFC3339, expStr)
				if err == nil && time.Now().After(expTime) {
					return fmt.Errorf("license expired at %s", expStr)
				}
			}
		}

		log.Printf("license: valid (customer: %s, type: %s)", info.CustomerName, info.LicenseType)
		return nil
	}

	return fmt.Errorf("SDK unreachable after 30 attempts: %w", lastErr)
}

// CheckLicenseFieldBool queries a boolean license field from the Replicated SDK.
// Returns the field value if reachable, or the fallback if the SDK is unavailable.
func CheckLicenseFieldBool(sdkServiceName, fieldName string, fallback bool) bool {
	endpoint := fmt.Sprintf("http://%s:3000/api/v1/license/fields/%s", sdkServiceName, fieldName)
	log.Printf("license: checking field %s at %s", fieldName, endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		log.Printf("license: create request: %v, using fallback %v", err, fallback)
		return fallback
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("license: SDK unreachable for field %s: %v, using fallback %v", fieldName, err, fallback)
		return fallback
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("license: SDK returned %d for field %s, using fallback %v", resp.StatusCode, fieldName, fallback)
		return fallback
	}

	var field LicenseField
	if err := json.NewDecoder(resp.Body).Decode(&field); err != nil {
		log.Printf("license: decode field %s: %v, using fallback %v", fieldName, err, fallback)
		return fallback
	}

	switch v := field.Value.(type) {
	case bool:
		log.Printf("license: field %s = %v", fieldName, v)
		return v
	case string:
		result := v == "true" || v == "1"
		log.Printf("license: field %s = %q (resolved to %v)", fieldName, v, result)
		return result
	default:
		log.Printf("license: field %s has unexpected type %T, using fallback %v", fieldName, field.Value, fallback)
		return fallback
	}
}
