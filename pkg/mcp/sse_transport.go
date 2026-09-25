package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// SSETransport implements Server-Sent Events transport for MCP
// Used by servers like Atlassian MCP that require SSE connections
type SSETransport struct {
	baseURL     string
	httpClient  *http.Client
	sessionID   string
	oauthToken  string
	connected   bool
	eventStream *http.Response
	eventReader *bufio.Reader
	mu          sync.Mutex
	eventChan   chan SSEEvent
	stopChan    chan struct{}
}

// SSEEvent represents a server-sent event
type SSEEvent struct {
	Event string
	Data  string
	ID    string
}

// NewSSETransport creates a new SSE transport
func NewSSETransport(baseURL string, timeout time.Duration) *SSETransport {
	return &SSETransport{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		eventChan: make(chan SSEEvent, 100),
		stopChan:  make(chan struct{}),
	}
}

// SetOAuthToken sets the OAuth bearer token for authenticated requests
func (t *SSETransport) SetOAuthToken(token string) {
	t.oauthToken = token
}

// Connect establishes the SSE connection
func (t *SSETransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return fmt.Errorf("already connected")
	}

	// Build the SSE endpoint URL
	sseURL := t.baseURL

	log.Info().Str("url", sseURL).Msg("Establishing SSE connection")

	// Create GET request for SSE stream
	req, err := http.NewRequestWithContext(ctx, "GET", sseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create SSE request: %w", err)
	}

	// Set SSE headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Include OAuth token if we have one
	if t.oauthToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.oauthToken))
	}

	// Make the request
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("SSE connection failed: %w", err)
	}

	// Check status
	if resp.StatusCode == http.StatusUnauthorized {
		wwwAuth := resp.Header.Get("WWW-Authenticate")
		resp.Body.Close()
		return fmt.Errorf("unauthorized: %s (status %d)", wwwAuth, resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return fmt.Errorf("SSE connection failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Extract session ID from response headers
	if sessionID := resp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		t.sessionID = sessionID
		log.Info().Str("session_id", sessionID).Msg("Received session ID from Mcp-Session-Id header")
	} else if sessionID := resp.Header.Get("X-Session-Id"); sessionID != "" {
		t.sessionID = sessionID
		log.Info().Str("session_id", sessionID).Msg("Received session ID from X-Session-Id header")
	}

	// Store the event stream
	t.eventStream = resp
	t.eventReader = bufio.NewReader(resp.Body)
	t.connected = true

	// Start reading events in background
	go t.readEvents()

	// If no sessionId from headers, try to extract from first SSE event
	// This is common in Atlassian and similar implementations
	if t.sessionID == "" {

		// Wait for first event with timeout
		timeout := time.NewTimer(5 * time.Second)
		defer timeout.Stop()

		select {
		case event := <-t.eventChan:
			// Try to extract sessionId from event
			if event.Event == "endpoint" || event.Event == "session" {
				// Parse event data for sessionId
				var eventData map[string]interface{}
				if err := json.Unmarshal([]byte(event.Data), &eventData); err == nil {
					if sid, ok := eventData["sessionId"].(string); ok {
						t.sessionID = sid
						log.Info().Str("session_id", sid).Msg("Received session ID from SSE event")
					} else if sid, ok := eventData["session_id"].(string); ok {
						t.sessionID = sid
						log.Info().Str("session_id", sid).Msg("Received session ID from SSE event")
					}
				}
			}
			// If still no sessionId, try to extract from endpoint event
			if t.sessionID == "" && event.Data != "" {
				// Sometimes the endpoint URL contains sessionId as query param
				if strings.Contains(event.Data, "sessionId=") {
					parts := strings.Split(event.Data, "sessionId=")
					if len(parts) > 1 {
						sid := strings.Split(parts[1], "&")[0]
						sid = strings.Split(sid, "\"")[0]
						t.sessionID = sid
						log.Info().Str("session_id", sid).Msg("Extracted session ID from endpoint URL")
					}
				}
			}
		case <-timeout.C:
			log.Warn().Msg("Timeout waiting for session ID from SSE stream")
		}
	}

	log.Info().Msg("SSE connection established")
	return nil
}

// readEvents reads server-sent events from the stream
func (t *SSETransport) readEvents() {
	defer func() {
		if t.eventStream != nil {
			t.eventStream.Body.Close()
		}
		// Don't close eventChan here - it might be in use
		// The Close() method will handle cleanup
	}()

	var currentEvent SSEEvent

	for {
		select {
		case <-t.stopChan:
			return
		default:
			line, err := t.eventReader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.Error().Err(err).Msg("Error reading SSE stream")
				}
				// Stream closed - exit gracefully
				return
			}

			line = strings.TrimRight(line, "\n\r")

			// Empty line signals end of event
			if line == "" {
				if currentEvent.Data != "" {
					// Only log non-keepalive events in debug mode
					if currentEvent.Event != "" && currentEvent.Event != "keepalive" {
						log.Debug().
							Str("event", currentEvent.Event).
							Msg("Received SSE event")
					}

					select {
					case t.eventChan <- currentEvent:
					case <-t.stopChan:
						return
					}
					currentEvent = SSEEvent{}
				}
				continue
			}

			// Parse event field
			if strings.HasPrefix(line, "event: ") {
				currentEvent.Event = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if currentEvent.Data != "" {
					currentEvent.Data += "\n" + data
				} else {
					currentEvent.Data = data
				}
			} else if strings.HasPrefix(line, "id: ") {
				currentEvent.ID = strings.TrimPrefix(line, "id: ")
			}
		}
	}
}

// SendRequest sends a JSON-RPC request and waits for response
func (t *SSETransport) SendRequest(ctx context.Context, request JSONRPCRequest) (*JSONRPCResponse, error) {
	if !t.connected {
		return nil, fmt.Errorf("transport not connected")
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build the message endpoint URL
	// For SSE, we typically POST to the same endpoint or a /message endpoint
	messageURL := t.baseURL

	// Add sessionId as query parameter if we have one
	if t.sessionID != "" {
		separator := "?"
		if strings.Contains(messageURL, "?") {
			separator = "&"
		}
		messageURL = fmt.Sprintf("%s%ssessionId=%s", messageURL, separator, t.sessionID)
	}

	// Send request via SSE (debug logging removed for cleaner output)

	// Create POST request to send message
	req, err := http.NewRequestWithContext(ctx, "POST", messageURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if t.oauthToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.oauthToken))
	}

	// Send the request
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// For SSE, the server might respond with 202 Accepted or 200 OK
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// For POST requests to SSE endpoints, response might be empty (202)
	// The actual response comes via the SSE stream
	if resp.StatusCode == http.StatusAccepted {
		// Wait for response from event stream
		return t.waitForResponse(ctx, request.ID)
	}

	// If we got 200 OK with a body, parse it
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if len(responseBody) == 0 {
		// Empty response, wait for event stream
		return t.waitForResponse(ctx, request.ID)
	}

	return parseJSONRPCResponse(responseBody)
}

// waitForResponse waits for a response from the SSE event stream
func (t *SSETransport) waitForResponse(ctx context.Context, requestID interface{}) (*JSONRPCResponse, error) {
	timeout := time.NewTimer(120 * time.Second) // Increased timeout for slow servers
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout.C:
			return nil, fmt.Errorf("timeout waiting for SSE response")
		case event, ok := <-t.eventChan:
			if !ok {
				return nil, fmt.Errorf("SSE event channel closed")
			}

			// Parse the event data as JSON-RPC response
			var response JSONRPCResponse
			if err := json.Unmarshal([]byte(event.Data), &response); err != nil {
				// Skip non-JSON-RPC events silently
				continue
			}

			// Check if this is the response we're waiting for
			// Handle type conversion for ID comparison (JSON unmarshals numbers as float64)
			responseIDMatches := false
			switch rid := response.ID.(type) {
			case int:
				if expID, ok := requestID.(int); ok {
					responseIDMatches = rid == expID
				}
			case float64:
				// JSON unmarshals numbers as float64
				if expID, ok := requestID.(int); ok {
					responseIDMatches = rid == float64(expID)
				} else if expID, ok := requestID.(float64); ok {
					responseIDMatches = rid == expID
				}
			case string:
				if expID, ok := requestID.(string); ok {
					responseIDMatches = rid == expID
				}
			default:
				// Fallback to direct comparison
				responseIDMatches = response.ID == requestID
			}

			if responseIDMatches {
				return &response, nil
			}

			// Continue waiting for the correct response
			// (skipping responses with mismatched IDs)
		}
	}
}

// SendNotification sends a JSON-RPC notification (no response expected)
func (t *SSETransport) SendNotification(ctx context.Context, notification JSONRPCRequest) error {
	if !t.connected {
		return fmt.Errorf("transport not connected")
	}

	requestBody, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Build the message endpoint URL
	messageURL := t.baseURL

	if t.sessionID != "" {
		separator := "?"
		if strings.Contains(messageURL, "?") {
			separator = "&"
		}
		messageURL = fmt.Sprintf("%s%ssessionId=%s", messageURL, separator, t.sessionID)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", messageURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if t.oauthToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.oauthToken))
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Notifications don't require successful response, just log errors
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		log.Warn().Int("status", resp.StatusCode).Bytes("body", body).Msg("Notification returned error status")
	}

	return nil
}

// Close closes the SSE connection
func (t *SSETransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}

	close(t.stopChan)

	if t.eventStream != nil {
		t.eventStream.Body.Close()
	}

	t.connected = false
	log.Info().Msg("SSE connection closed")
	return nil
}

// IsConnected returns true if the transport is connected
func (t *SSETransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}
