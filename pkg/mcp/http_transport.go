package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// HTTPTransport implements the Transport interface for HTTP-based MCP servers
type HTTPTransport struct {
	baseURL    string
	httpClient *http.Client
	sessionID  string
	oauthToken string
	connected  bool
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(baseURL string, timeout time.Duration) *HTTPTransport {
	// Normalize the URL for FastMCP servers
	// If the URL doesn't have a path component (or just has /), append /mcp/v1
	parsedURL, err := url.Parse(baseURL)
	if err == nil {
		path := parsedURL.Path
		// If no path or just root, and doesn't already contain /mcp, add /mcp/v1
		if (path == "" || path == "/") && !strings.Contains(baseURL, "/mcp") {
			baseURL = strings.TrimSuffix(baseURL, "/") + "/mcp/v1"
			log.Debug().Str("original", parsedURL.String()).Str("normalized", baseURL).Msg("Normalized URL for FastMCP")
		}
	}

	return &HTTPTransport{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// SetOAuthToken sets the OAuth bearer token for authenticated requests
func (t *HTTPTransport) SetOAuthToken(token string) {
	t.oauthToken = token
}

// Connect establishes the HTTP connection (no-op for HTTP, connection is per-request)
func (t *HTTPTransport) Connect(ctx context.Context) error {
	t.connected = true
	return nil
}

// SendRequest sends a JSON-RPC request over HTTP and returns the response
func (t *HTTPTransport) SendRequest(ctx context.Context, request JSONRPCRequest) (*JSONRPCResponse, error) {
	if !t.connected {
		return nil, fmt.Errorf("transport not connected")
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL with sessionId as query parameter if we have one
	requestURL := t.baseURL
	if t.sessionID != "" {
		separator := "?"
		if strings.Contains(t.baseURL, "?") {
			separator = "&"
		}
		requestURL = fmt.Sprintf("%s%ssessionId=%s", t.baseURL, separator, t.sessionID)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set required headers per MCP spec
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	// Also include session ID in header (some servers use this)
	if t.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", t.sessionID)
	}

	// Include OAuth token if we have one
	if t.oauthToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.oauthToken))
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle 401 Unauthorized (may need OAuth)
	if resp.StatusCode == http.StatusUnauthorized {
		wwwAuth := resp.Header.Get("WWW-Authenticate")
		return nil, fmt.Errorf("unauthorized: %s (status %d)", wwwAuth, resp.StatusCode)
	}

	// Accept 200 OK and 202 Accepted (used by Streamable HTTP)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Capture session ID from response headers
	if sessionID := resp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		t.sessionID = sessionID
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle Server-Sent Events format
	responseStr := string(responseBody)
	if strings.HasPrefix(responseStr, "event:") {
		jsonData, err := parseSSEResponse(responseStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSE response: %w", err)
		}
		responseBody = []byte(jsonData)
	}

	return parseJSONRPCResponse(responseBody)
}

// SendNotification sends a JSON-RPC notification over HTTP (no response expected)
func (t *HTTPTransport) SendNotification(ctx context.Context, notification JSONRPCRequest) error {
	if !t.connected {
		return fmt.Errorf("transport not connected")
	}

	requestBody, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Build URL with sessionId as query parameter if we have one
	requestURL := t.baseURL
	if t.sessionID != "" {
		separator := "?"
		if strings.Contains(t.baseURL, "?") {
			separator = "&"
		}
		requestURL = fmt.Sprintf("%s%ssessionId=%s", t.baseURL, separator, t.sessionID)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	if t.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", t.sessionID)
	}

	if t.oauthToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.oauthToken))
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Log any errors but don't fail (notifications don't require responses)
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		log.Warn().Int("status", resp.StatusCode).Bytes("body", body).Msg("Notification returned error status")
	}

	return nil
}

// Close closes the HTTP connection (no-op for HTTP)
func (t *HTTPTransport) Close() error {
	t.connected = false
	return nil
}

// IsConnected returns true if the transport is connected
func (t *HTTPTransport) IsConnected() bool {
	return t.connected
}

// parseSSEResponse extracts JSON data from Server-Sent Events format
func parseSSEResponse(sseData string) (string, error) {
	lines := strings.Split(sseData, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			return strings.TrimPrefix(line, "data: "), nil
		}
	}
	return "", fmt.Errorf("no data field found in SSE response")
}
