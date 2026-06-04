// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package ui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/aibom"
	"github.com/accuknox/accuknox-cli-v2/pkg/cbom"
	"github.com/accuknox/accuknox-cli-v2/pkg/sign"
	"gopkg.in/yaml.v3"
)

// Server is the knoxctl embedded web UI HTTP server.
type Server struct {
	addr    string
	version string
	mux     *http.ServeMux
}

// ──────────────────────────────────────────────────────────────────────────────
// Persistent configuration
// ──────────────────────────────────────────────────────────────────────────────

// AppConfig is the persistent application configuration stored in
// ~/.knoxctl-cfg.yaml (Linux/macOS) or knoxctl-cfg.yaml (Windows).
type AppConfig struct {
	BOM       BOMSettings    `yaml:"bom"       json:"bom"`
	Dashboard DashboardStats `yaml:"dashboard" json:"dashboard"`
}

// BOMSettings holds the AccuKnox control-plane connection parameters used to
// publish Bill of Materials artefacts.
type BOMSettings struct {
	ControlPlane string `yaml:"control_plane" json:"control_plane"`
	Project      string `yaml:"project"       json:"project"`
	Label        string `yaml:"label"         json:"label"`
	Token        string `yaml:"token"         json:"token"`
}

// DashboardStats holds the persistent operation counters shown on the dashboard.
type DashboardStats struct {
	SBOM  int `yaml:"sbom"  json:"sbom"`
	CBOM  int `yaml:"cbom"  json:"cbom"`
	AIBOM int `yaml:"aibom" json:"aibom"`
	Scan  int `yaml:"scan"  json:"scan"`
}

// configFilePath returns the path to the knoxctl config file.
func configFilePath() string {
	if runtime.GOOS == "windows" {
		return "knoxctl-cfg.yaml"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".knoxctl-cfg.yaml")
}

// loadAppConfig reads and parses the config file; returns zero value on any error.
func loadAppConfig() AppConfig {
	var cfg AppConfig
	data, err := os.ReadFile(configFilePath()) // #nosec G304
	if err != nil {
		return cfg
	}
	_ = yaml.Unmarshal(data, &cfg)
	return cfg
}

// saveAppConfig writes cfg to the config file with mode 0600.
func saveAppConfig(cfg AppConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(configFilePath(), data, 0600) // #nosec G306
}

// NewServer creates a new Server listening on addr (e.g. "0.0.0.0:10100").
func NewServer(addr, version string) *Server {
	s := &Server{addr: addr, version: version}
	s.mux = http.NewServeMux()
	s.registerRoutes()
	return s
}

// Start starts the HTTP server and opens the UI in the default browser.
func (s *Server) Start() error {
	url := "http://localhost:" + portFrom(s.addr)
	fmt.Printf("knoxctl UI  →  %s\n", url)
	fmt.Printf("Listening on %s  (Ctrl-C to stop)\n", s.addr)

	// Open browser after a short delay so the server is ready.
	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser(url)
	}()

	srv := &http.Server{
		Addr:              s.addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

// registerRoutes wires all HTTP handlers.
func (s *Server) registerRoutes() {
	// Static assets — serve the embedded SPA for every non-API route.
	sub, _ := fs.Sub(StaticFS, "static")
	fileServer := http.FileServer(http.FS(sub))
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// All non-API paths fall through to index.html (SPA routing).
		if r.URL.Path != "/" {
			_, err := fs.Stat(sub, strings.TrimPrefix(r.URL.Path, "/"))
			if err != nil {
				r.URL.Path = "/"
			}
		}
		fileServer.ServeHTTP(w, r)
	})

	// API — version
	s.mux.HandleFunc("/api/version", cors(s.handleVersion))

	// API — config (GET=load, POST=save)
	s.mux.HandleFunc("/api/config", cors(s.handleConfig))
	s.mux.HandleFunc("/api/config/projects", cors(s.handleFetchProjects))
	s.mux.HandleFunc("/api/config/labels", cors(s.handleFetchLabels))

	// API — SBOM
	s.mux.HandleFunc("/api/sbom/generate", cors(s.handleSBOM))
	s.mux.HandleFunc("/api/sbom/publish", cors(func(w http.ResponseWriter, r *http.Request) {
		s.handlePublishBOM(w, r, "sbom")
	}))

	// API — CBOM publish
	s.mux.HandleFunc("/api/cbom/publish", cors(func(w http.ResponseWriter, r *http.Request) {
		s.handlePublishBOM(w, r, "cbom")
	}))

	// API — AIBOM publish
	s.mux.HandleFunc("/api/aibom/publish", cors(func(w http.ResponseWriter, r *http.Request) {
		s.handlePublishBOM(w, r, "aibom")
	}))

	// API — CBOM
	s.mux.HandleFunc("/api/cbom/source", cors(s.handleCBOMSource))
	s.mux.HandleFunc("/api/cbom/image", cors(s.handleCBOMImage))

	// API — AIBOM
	s.mux.HandleFunc("/api/aibom/generate", cors(s.handleAIBOM))

	// API — generic CLI runner (image-scan, probe, vm, sbom)
	s.mux.HandleFunc("/api/run", cors(s.handleRun))
}

// ──────────────────────────────────────────────────────────────────────────────
// Handlers
// ──────────────────────────────────────────────────────────────────────────────

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{
		"version": s.version,
		"sha256":  binarySHA256(),
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// binarySHA256 computes the SHA-256 digest of the running knoxctl binary.
// Returns an empty string on any error (e.g. the binary was deleted after launch).
func binarySHA256() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	f, err := os.Open(exe) // #nosec G304
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// handleConfig serves GET (load) and POST (save) for the persistent app config.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, loadAppConfig())
	case http.MethodPost:
		var cfg AppConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeErr(w, "invalid request: "+err.Error())
			return
		}
		if err := saveAppConfig(cfg); err != nil {
			writeErr(w, "failed to save config: "+err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "saved"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFetchProjects proxies a project-list request to the AccuKnox control
// plane and returns the raw JSON response to the browser.
func (s *Server) handleFetchProjects(w http.ResponseWriter, r *http.Request) {
	cfg := loadAppConfig()
	bs := cfg.BOM
	if bs.ControlPlane == "" || bs.Token == "" {
		writeErr(w, "control plane URL and token must be saved in BOM Settings first")
		return
	}

	apiURL := strings.TrimRight(bs.ControlPlane, "/") +
		"/api/v1/sbom/projects?page=1&page_size=100"

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, nil) // #nosec G107
	if err != nil {
		writeErr(w, "failed to build request: "+err.Error())
		return
	}
	req.Header.Set("Authorization", "Bearer "+bs.Token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeErr(w, "failed to fetch projects: "+err.Error())
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		writeErr(w, fmt.Sprintf("control plane returned HTTP %d: %s", resp.StatusCode, string(body)))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

// handleFetchLabels proxies a labels request to the AccuKnox control plane.
func (s *Server) handleFetchLabels(w http.ResponseWriter, r *http.Request) {
	cfg := loadAppConfig()
	bs := cfg.BOM
	if bs.ControlPlane == "" || bs.Token == "" {
		writeErr(w, "control plane URL and token must be saved in BOM Settings first")
		return
	}

	apiURL := strings.TrimRight(bs.ControlPlane, "/") + "/api/v1/labels-mini"

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, nil) // #nosec G107
	if err != nil {
		writeErr(w, "failed to build request: "+err.Error())
		return
	}
	req.Header.Set("Authorization", "Bearer "+bs.Token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeErr(w, "failed to fetch labels: "+err.Error())
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		writeErr(w, fmt.Sprintf("control plane returned HTTP %d: %s", resp.StatusCode, string(body)))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

// handlePublishBOM uploads a BOM JSON to the AccuKnox control plane.
// bomType is one of "sbom", "cbom", or "aibom" and determines the API path.
func (s *Server) handlePublishBOM(w http.ResponseWriter, r *http.Request, bomType string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		BOM string `json:"bom"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, "invalid request: "+err.Error())
		return
	}
	if req.BOM == "" {
		writeErr(w, "bom payload is empty")
		return
	}

	cfg := loadAppConfig()
	bs := cfg.BOM
	if bs.ControlPlane == "" || bs.Project == "" || bs.Label == "" || bs.Token == "" {
		writeErr(w, "BOM settings are incomplete — configure them in Settings")
		return
	}

	// Build multipart body: field name "file", filename "<type>.json".
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", bomType+".json")
	if err != nil {
		writeErr(w, "failed to create multipart field: "+err.Error())
		return
	}
	if _, err := fw.Write([]byte(req.BOM)); err != nil {
		writeErr(w, "failed to write BOM data: "+err.Error())
		return
	}
	mw.Close()

	// All BOM types use the same upload endpoint.
	apiURL := strings.TrimRight(bs.ControlPlane, "/") +
		"/api/v1/sbom/bomfiles/upload" +
		"?project_id=" + url.QueryEscape(bs.Project) +
		"&label_id=" + url.QueryEscape(bs.Label) +
		"&save_to_s3=true"

	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, apiURL, &buf) // #nosec G107
	if err != nil {
		writeErr(w, "failed to build request: "+err.Error())
		return
	}
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+bs.Token)
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		writeErr(w, "publish request failed: "+err.Error())
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		writeErr(w, fmt.Sprintf("control plane returned HTTP %d: %s", resp.StatusCode, string(body)))
		return
	}
	writeJSON(w, map[string]interface{}{
		"status": "published",
		"code":   resp.StatusCode,
		"body":   string(body),
	})
}

func (s *Server) handleSBOM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Source  string `json:"source"`
		Scheme  string `json:"scheme"`
		Format  string `json:"format"`
		Exclude string `json:"exclude"`
		signReq
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, "invalid request: "+err.Error())
		return
	}
	if req.Source == "" {
		req.Source = "."
	}
	if req.Format == "" {
		req.Format = "cyclonedx-json"
	}

	send, flush, ok := sseInit(w)
	if !ok {
		return
	}

	send("progress", progress(10, "Initialising Software BOM generation…"))
	flush()
	if ctxDone(r.Context(), send, flush) {
		return
	}

	// pkgscan writes output via "-o format=file".
	tmpDir, err := os.MkdirTemp("", "knoxctl-sbom-*")
	if err != nil {
		send("error", errMsg(err))
		flush()
		return
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to remove temp dir %s: %v\n", tmpDir, err)
		}
	}()
	outFile := filepath.Join(tmpDir, "sbom.json")

	// Prefix source with scheme when explicitly chosen.
	source := req.Source
	if req.Scheme != "" {
		source = req.Scheme + ":" + source
	}

	args := []string{"pkgscan", "scan", source, "-o", req.Format + "=" + outFile}

	// Use basename as the SBOM source name when the input looks like a filesystem path,
	// so the output doesn't embed full local directory paths.
	scheme := req.Scheme
	if scheme == "" {
		scheme = "dir" // default when no scheme is set
	}
	if scheme == "dir" || scheme == "file" || scheme == "oci-dir" || scheme == "oci-archive" {
		if name := filepath.Base(req.Source); name != "" && name != "." {
			args = append(args, "--source-name", name)
		}
	}
	for _, pat := range strings.Split(req.Exclude, ",") {
		if pat = strings.TrimSpace(pat); pat != "" {
			args = append(args, "--exclude", pat)
		}
	}

	send("progress", progress(20, "Scanning "+source+" for packages…"))
	flush()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, resolveKnoxctl(), args...) // #nosec G204
	cmd.Stdout = &lineWriter{send: send, flush: flush, event: "log"}
	cmd.Stderr = &lineWriter{send: send, flush: flush, event: "log"}

	if err := cmd.Start(); err != nil {
		send("error", errMsg(err))
		flush()
		return
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil && ctx.Err() == nil {
			send("error", errMsg(err))
			flush()
			return
		}
		if ctx.Err() != nil {
			return
		}
	case <-ctx.Done():
		<-done
		return
	}

	send("progress", progress(80, "Reading SBOM output…"))
	flush()

	bomJSON, err := os.ReadFile(outFile) // #nosec G304 — path inside os.MkdirTemp dir
	if err != nil {
		send("error", errMsg(fmt.Errorf("pkgscan produced no output file: %w", err)))
		flush()
		return
	}

	// Count top-level components (works for cyclonedx-json, pkgscan-json, and spdx).
	var parsed struct {
		Components *[]json.RawMessage `json:"components"` // CycloneDX
		Artifacts  *[]json.RawMessage `json:"artifacts"`  // pkgscan native
		Packages   *[]json.RawMessage `json:"packages"`   // SPDX
	}
	count := 0
	if json.Unmarshal(bomJSON, &parsed) == nil {
		switch {
		case parsed.Components != nil:
			count = len(*parsed.Components)
		case parsed.Artifacts != nil:
			count = len(*parsed.Artifacts)
		case parsed.Packages != nil:
			count = len(*parsed.Packages)
		}
	}

	// Rebrand tool metadata: replace syft/Anchore references with knoxctl/AccuKnox.
	bomJSON = rebrandSBOM(bomJSON)

	send("complete", buildComplete(count, bomJSON, req.signReq))
	flush()
}

// sbomReplacer performs all vendor-name substitutions in a single pass over the
// JSON string, avoiding the N×size memory spike of chained bytes.ReplaceAll calls.
// Patterns are listed most-specific first so shorter prefixes don't shadow them.
var sbomReplacer = strings.NewReplacer(
	"Anchore, Inc.", "AccuKnox",
	"Anchore Inc.", "AccuKnox",
	"github.com/anchore/syft", "github.com/accuknox/knoxctl",
	"https://github.com/anchore/syft", "https://github.com/accuknox/knoxctl",
	"github.com/anchore", "github.com/accuknox",
	"Anchore", "AccuKnox",
	"anchore", "accuknox",
	"\"name\":\"syft\"", "\"name\":\"knoxctl\"",
	`"syft"`, `"knoxctl"`,
	"syft-", "knoxctl-",
	"syft/", "knoxctl/",
	"Syft", "knoxctl",
	"syft", "knoxctl",
)

// rebrandSBOM replaces vendor-specific tool names in an SBOM JSON with AccuKnox branding.
func rebrandSBOM(data []byte) []byte {
	return []byte(sbomReplacer.Replace(string(data)))
}

func (s *Server) handleCBOMSource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Path        string `json:"path"`
		Name        string `json:"name"`
		Group       string `json:"group"`
		Version     string `json:"version"`
		Description string `json:"description"`
		License     string `json:"license"`
		signReq
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, "invalid request: "+err.Error())
		return
	}
	if req.Path == "" {
		req.Path = "."
	}
	// Default the component name to the basename of the path so full local
	// directory paths don't leak into the BOM metadata.
	if req.Name == "" {
		if base := filepath.Base(req.Path); base != "" && base != "." {
			req.Name = base
		}
	}

	send, flush, ok := sseInit(w)
	if !ok {
		return
	}

	send("progress", progress(10, "Initialising source scanner…"))
	flush()
	if ctxDone(r.Context(), send, flush) {
		return
	}

	opts := &cbom.Options{
		Path:        req.Path,
		Name:        req.Name,
		Group:       req.Group,
		Version:     req.Version,
		Description: req.Description,
		License:     req.License,
		Format:      "json",
	}

	send("progress", progress(30, "Scanning Go source files for cryptographic imports…"))
	flush()

	bom, err := cbom.GenerateFromSource(opts)
	if err != nil {
		send("error", errMsg(err))
		flush()
		return
	}
	if ctxDone(r.Context(), send, flush) {
		return
	}

	send("progress", progress(80, "Building Cryptography BOM…"))
	flush()

	out, err := json.MarshalIndent(bom, "", "  ")
	if err != nil {
		send("error", errMsg(err))
		flush()
		return
	}

	count := cbom.ComponentCount(bom)
	send("complete", buildComplete(count, out, req.signReq))
	flush()
}

func (s *Server) handleCBOMImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Image   string `json:"image"`
		Name    string `json:"name"`
		Plugins string `json:"plugins"`
		Ignore  string `json:"ignore"`
		signReq
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, "invalid request: "+err.Error())
		return
	}
	if req.Image == "" {
		writeErr(w, "image is required")
		return
	}

	send, flush, ok := sseInit(w)
	if !ok {
		return
	}

	send("progress", progress(10, "Initialising image scanner…"))
	flush()
	if ctxDone(r.Context(), send, flush) {
		return
	}

	opts := &cbom.Options{
		Image:   req.Image,
		Name:    req.Name,
		Plugins: req.Plugins,
		Ignore:  req.Ignore,
		Format:  "json",
	}

	send("progress", progress(30, "Pulling and scanning container image…"))
	flush()

	bom, err := cbom.GenerateFromImage(opts)
	if err != nil {
		send("error", errMsg(err))
		flush()
		return
	}
	if ctxDone(r.Context(), send, flush) {
		return
	}

	send("progress", progress(80, "Building Cryptography BOM…"))
	flush()

	out, err := json.MarshalIndent(bom, "", "  ")
	if err != nil {
		send("error", errMsg(err))
		flush()
		return
	}

	count := cbom.ComponentCount(bom)
	send("complete", buildComplete(count, out, req.signReq))
	flush()
}

func (s *Server) handleAIBOM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		// Common
		Source       string `json:"source"` // "huggingface" (default) or "bedrock"
		Name         string `json:"name"`
		Version      string `json:"version"`
		Manufacturer string `json:"manufacturer"`
		// HuggingFace
		ModelID string `json:"modelId"`
		Token   string `json:"token"`
		// AWS Bedrock
		Region                string `json:"region"`
		UseDefaultCredentials bool   `json:"useDefaultCredentials"`
		AccessKeyID           string `json:"accessKeyId"`
		SecretAccessKey       string `json:"secretAccessKey"`
		SessionToken          string `json:"sessionToken"`
		signReq
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, "invalid request: "+err.Error())
		return
	}
	if req.Source == "" {
		req.Source = "huggingface"
	}

	send, flush, ok := sseInit(w)
	if !ok {
		return
	}

	send("progress", progress(10, "Connecting to model registry…"))
	flush()
	if ctxDone(r.Context(), send, flush) {
		return
	}

	if req.Source == "bedrock" {
		if req.Region == "" {
			send("error", errMsg(fmt.Errorf("region is required for AWS Bedrock")))
			flush()
			return
		}
		send("progress", progress(30, "Fetching AWS Bedrock model catalog…"))
		flush()

		bedrockOpts := &aibom.BedrockOptions{
			Region:                req.Region,
			UseDefaultCredentials: req.UseDefaultCredentials || req.AccessKeyID == "",
			AccessKeyID:           req.AccessKeyID,
			SecretAccessKey:       req.SecretAccessKey,
			SessionToken:          req.SessionToken,
			ModelID:               req.ModelID,
			Name:                  req.Name,
			Version:               req.Version,
			Manufacturer:          req.Manufacturer,
			Format:                "json",
		}

		cdxBOM, err := aibom.GenerateFromBedrock(bedrockOpts)
		if err != nil {
			send("error", errMsg(err))
			flush()
			return
		}
		if ctxDone(r.Context(), send, flush) {
			return
		}

		send("progress", progress(80, "Building AI/ML BOM…"))
		flush()

		out, err := json.MarshalIndent(cdxBOM, "", "  ")
		if err != nil {
			send("error", errMsg(err))
			flush()
			return
		}
		count := aibom.ModelCount(cdxBOM)
		send("complete", buildComplete(count, out, req.signReq))
		flush()
		return
	}

	// HuggingFace (default)
	if req.ModelID == "" {
		send("error", errMsg(fmt.Errorf("modelId is required for HuggingFace")))
		flush()
		return
	}
	// Default the component name to the full model ID (org/model) so that
	// provenance is clear in the BOM metadata.
	if req.Name == "" {
		req.Name = req.ModelID
	}

	opts := &aibom.Options{
		ModelID:      req.ModelID,
		Token:        req.Token,
		Name:         req.Name,
		Version:      req.Version,
		Manufacturer: req.Manufacturer,
		Format:       "json",
	}

	send("progress", progress(30, "Fetching model metadata…"))
	flush()

	cdxBOM, err := aibom.Generate(opts)
	if err != nil {
		send("error", errMsg(err))
		flush()
		return
	}
	if ctxDone(r.Context(), send, flush) {
		return
	}

	send("progress", progress(80, "Building AI/ML BOM…"))
	flush()

	out, err := json.MarshalIndent(cdxBOM, "", "  ")
	if err != nil {
		send("error", errMsg(err))
		flush()
		return
	}

	count := aibom.ModelCount(cdxBOM)
	send("complete", buildComplete(count, out, req.signReq))
	flush()
}

// ctxDone returns true and sends an SSE error event if the request context has
// been cancelled (e.g. because the client aborted the fetch).
func ctxDone(ctx context.Context, send func(string, interface{}), flush func()) bool {
	if ctx.Err() == nil {
		return false
	}
	send("error", errMsg(ctx.Err()))
	flush()
	return true
}

// ──────────────────────────────────────────────────────────────────────────────
// Signing helpers
// ──────────────────────────────────────────────────────────────────────────────

// signReq holds the signing fields that every BOM handler accepts.
type signReq struct {
	SignEnabled     bool   `json:"signEnabled"`
	SignGenerateKey bool   `json:"signGenerateKey"`
	SignKeyRef      string `json:"signKeyRef"`
	SignPassword    string `json:"signPassword"`
}

// buildComplete builds the SSE "complete" payload.  When signing is requested
// it calls sign.SignBytes in-memory and appends the result; the JSON
// payload will contain "signed", "signature", and (for generated keys) "pubKey".
func buildComplete(count int, bomJSON []byte, sr signReq) map[string]interface{} {
	payload := map[string]interface{}{
		"count":  count,
		"result": string(bomJSON),
	}
	if !sr.SignEnabled {
		return payload
	}
	opts := &sign.Options{
		Enabled:     true,
		GenerateKey: sr.SignGenerateKey,
		KeyRef:      sr.SignKeyRef,
		Password:    sr.SignPassword,
	}
	sigB64, pubKeyPEM, err := sign.SignBytes(bomJSON, opts)
	if err != nil {
		payload["signError"] = err.Error()
		return payload
	}
	payload["signed"] = true
	payload["signature"] = sigB64
	if len(pubKeyPEM) > 0 {
		payload["pubKey"] = string(pubKeyPEM)
	}
	return payload
}

// handleRun executes an arbitrary knoxctl sub-command and streams its
// stdout/stderr line-by-line as SSE progress events.  Used for operations that
// are best delegated to the CLI (image-scan, probe, vm-onboard, etc.).
func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Args []string `json:"args"` // knoxctl subcommand args, e.g. ["probe", "--full"]
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, "invalid request: "+err.Error())
		return
	}
	if len(req.Args) == 0 {
		writeErr(w, "args are required")
		return
	}

	send, flush, ok := sseInit(w)
	if !ok {
		return
	}

	self := resolveKnoxctl()

	// Derive a timeout context from the *request* context so that an
	// AbortController on the client side (which closes the HTTP connection
	// and therefore cancels r.Context()) propagates to the subprocess.
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, self, req.Args...) // #nosec G204
	cmd.Stdout = &lineWriter{send: send, flush: flush, event: "log"}
	cmd.Stderr = &lineWriter{send: send, flush: flush, event: "log"}

	send("progress", progress(10, "Running: knoxctl "+strings.Join(req.Args, " ")))
	flush()

	if err := cmd.Start(); err != nil {
		send("error", errMsg(err))
		flush()
		return
	}

	// Wait for the process to finish in a goroutine so we can also listen for
	// context cancellation (client disconnect / Stop button / timeout).
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		// Process exited normally (or with an error).
		if err != nil && ctx.Err() == nil {
			send("error", errMsg(err))
			flush()
		} else if ctx.Err() == nil {
			send("complete", map[string]interface{}{"message": "Command completed successfully."})
			flush()
		}
		// If ctx is already done the client is gone — no point writing.

	case <-ctx.Done():
		// Client disconnected or timeout hit.
		// exec.CommandContext will send SIGKILL; wait for it to exit.
		<-done
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// SSE helpers
// ──────────────────────────────────────────────────────────────────────────────

type sendFn = func(event string, data interface{})

func sseInit(w http.ResponseWriter) (sendFn, func(), bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return nil, nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	send := func(event string, data interface{}) {
		b, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
	}
	flush := func() { flusher.Flush() }
	return send, flush, true
}

func progress(pct int, msg string) map[string]interface{} {
	return map[string]interface{}{"percent": pct, "message": msg}
}

func errMsg(err error) map[string]interface{} {
	return map[string]interface{}{"message": err.Error()}
}

// lineWriter streams each written chunk as an SSE event.
type lineWriter struct {
	send  sendFn
	flush func()
	event string
}

func (lw *lineWriter) Write(p []byte) (int, error) {
	lines := strings.Split(strings.TrimRight(string(p), "\n"), "\n")
	for _, line := range lines {
		if line != "" {
			lw.send(lw.event, map[string]string{"line": line})
			lw.flush()
		}
	}
	return len(p), nil
}

// ──────────────────────────────────────────────────────────────────────────────
// HTTP helpers
// ──────────────────────────────────────────────────────────────────────────────

func cors(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func writeErr(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		http.Error(w, "failed to encode error response", http.StatusInternalServerError)
	}
}

// portFrom extracts the port number from "host:port".
func portFrom(addr string) string {
	parts := strings.SplitN(addr, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return addr
}

// resolveKnoxctl returns the path to the knoxctl binary.
// It checks the current working directory first, then falls back to PATH.
func resolveKnoxctl() string {
	binary := "knoxctl"
	if runtime.GOOS == "windows" {
		binary = "knoxctl.exe"
	}
	if cwd, err := os.Getwd(); err == nil {
		local := filepath.Join(cwd, binary)
		if info, err := os.Stat(local); err == nil && !info.IsDir() {
			return local
		}
	}
	if path, err := exec.LookPath(binary); err == nil {
		return path
	}
	return binary
}

// openBrowser opens url in the system default browser.
func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	_ = exec.Command(cmd, args...).Start() // #nosec G204
}
