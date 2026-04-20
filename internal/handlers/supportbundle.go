package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

type SupportBundleHandler struct {
	SDKServiceName string
}

func NewSupportBundleHandler(sdkServiceName string) *SupportBundleHandler {
	return &SupportBundleHandler{
		SDKServiceName: sdkServiceName,
	}
}

func (h *SupportBundleHandler) Generate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}

	log.Println("support-bundle: generating bundle via support-bundle --load-cluster-specs")

	// Run support-bundle binary to collect from cluster specs (our Secret)
	outDir, err := os.MkdirTemp("", "support-bundle-*")
	if err != nil {
		log.Printf("support-bundle: create temp dir: %v", err)
		http.Error(w, "Failed to generate support bundle", 500)
		return
	}
	defer os.RemoveAll(outDir)

	outFile := filepath.Join(outDir, "bundle.tar.gz")
	cmd := exec.CommandContext(r.Context(),
		"support-bundle",
		"--load-cluster-specs",
		"--interactive=false",
		fmt.Sprintf("--output=%s", outFile),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Printf("support-bundle: collection failed: %v", err)
		http.Error(w, "Failed to collect support bundle", 500)
		return
	}

	// Read the generated bundle
	bundle, err := os.ReadFile(outFile)
	if err != nil {
		log.Printf("support-bundle: read bundle file: %v", err)
		http.Error(w, "Failed to read support bundle", 500)
		return
	}

	log.Printf("support-bundle: collected %d bytes, uploading to SDK", len(bundle))

	// Upload to Replicated SDK
	bundleID, err := h.uploadBundle(r, bundle)
	if err != nil {
		log.Printf("support-bundle: upload failed: %v", err)
		http.Error(w, "Failed to upload support bundle", 500)
		return
	}

	log.Printf("support-bundle: uploaded successfully (id: %s)", bundleID)

	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/admin"
	}
	http.Redirect(w, r, ref+"?flash=Support+bundle+uploaded+successfully", http.StatusSeeOther)
}

func (h *SupportBundleHandler) uploadBundle(r *http.Request, bundle []byte) (string, error) {
	endpoint := fmt.Sprintf("http://%s:3000/api/v1/supportbundle", h.SDKServiceName)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(bundle))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/gzip")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(bundle)))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("SDK returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		BundleID string `json:"bundleId"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return result.BundleID, nil
}
