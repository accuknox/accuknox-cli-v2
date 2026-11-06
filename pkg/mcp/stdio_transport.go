package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// STDIOTransport implements the Transport interface for STDIO-based MCP servers
type STDIOTransport struct {
	command   string
	args      []string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	reader    *bufio.Reader
	connected bool
	mu        sync.Mutex

	// Response channels for matching requests to responses
	pendingRequests map[interface{}]chan *JSONRPCResponse
	requestsMu      sync.Mutex
}

// NewSTDIOTransport creates a new STDIO transport
func NewSTDIOTransport(command string, args ...string) *STDIOTransport {
	return &STDIOTransport{
		command:         command,
		args:            args,
		pendingRequests: make(map[interface{}]chan *JSONRPCResponse),
	}
}

// Connect starts the MCP server process and establishes STDIO communication
func (t *STDIOTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return fmt.Errorf("already connected")
	}

	// Create the command
	t.cmd = exec.CommandContext(ctx, t.command, t.args...)

	// Setup stdin
	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	t.stdin = stdin

	// Setup stdout
	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	t.stdout = stdout
	t.reader = bufio.NewReader(stdout)

	// Setup stderr
	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	t.stderr = stderr

	// Start the process
	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server process: %w", err)
	}

	t.connected = true

	// Start reading responses in background
	go t.readResponses()
	go t.readStderr()

	log.Info().Str("command", t.command).Msg("Started MCP server process")
	return nil
}

// SendRequest sends a JSON-RPC request via STDIO and waits for the response
func (t *STDIOTransport) SendRequest(ctx context.Context, request JSONRPCRequest) (*JSONRPCResponse, error) {
	if !t.connected {
		return nil, fmt.Errorf("transport not connected")
	}

	// Create a channel for this request's response
	responseChan := make(chan *JSONRPCResponse, 1)

	t.requestsMu.Lock()
	t.pendingRequests[request.ID] = responseChan
	t.requestsMu.Unlock()

	// Cleanup on exit
	defer func() {
		t.requestsMu.Lock()
		delete(t.pendingRequests, request.ID)
		t.requestsMu.Unlock()
		close(responseChan)
	}()

	// Marshal and send the request
	requestData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Write to stdin
	log.Debug().
		Interface("request_id", request.ID).
		Str("method", request.Method).
		Str("request_json", string(requestData)).
		Msg("Sending JSON-RPC request to server")

	if _, err := t.stdin.Write(append(requestData, '\n')); err != nil {
		return nil, fmt.Errorf("failed to write request to stdin: %w", err)
	}

	// Wait for response or context cancellation
	select {
	case response := <-responseChan:
		return response, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("request cancelled: %w", ctx.Err())
	}
}

// SendNotification sends a JSON-RPC notification via STDIO (no response expected)
func (t *STDIOTransport) SendNotification(ctx context.Context, notification JSONRPCRequest) error {
	if !t.connected {
		return fmt.Errorf("transport not connected")
	}

	// Marshal and send the notification
	notificationData, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Write to stdin
	if _, err := t.stdin.Write(append(notificationData, '\n')); err != nil {
		return fmt.Errorf("failed to write notification to stdin: %w", err)
	}

	return nil
}

// Close closes the STDIO connection and terminates the process
func (t *STDIOTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}

	// Close stdin to signal the process to exit
	if t.stdin != nil {
		t.stdin.Close()
	}

	// Wait for process to exit (with timeout)
	if t.cmd != nil && t.cmd.Process != nil {
		// Give it some time to exit gracefully
		done := make(chan error, 1)
		go func() {
			done <- t.cmd.Wait()
		}()

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		select {
		case <-done:
			// Process exited
		case <-timeoutCtx.Done():
			// Timeout, force kill
			if err := t.cmd.Process.Kill(); err != nil {
				log.Warn().Err(err).Msg("Failed to kill MCP server process")
			}
		}
	}

	t.connected = false
	log.Info().Msg("Closed MCP server process")
	return nil
}

// IsConnected returns true if the transport is connected
func (t *STDIOTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}

// readResponses continuously reads JSON-RPC responses from stdout
func (t *STDIOTransport) readResponses() {
	for t.connected {
		line, err := t.reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				log.Warn().Err(err).Msg("Error reading from stdout")
			}
			return
		}

		// Skip empty lines
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// Try to parse as JSON-RPC response
		var response JSONRPCResponse
		if err := json.Unmarshal(line, &response); err != nil {
			// Skip non-JSON lines (e.g., server banners that violate MCP spec)
			// These should be on stderr but some servers write them to stdout
			log.Debug().Str("line", string(line)).Msg("Skipping non-JSON line from stdout (should be on stderr)")
			continue
		}

		log.Debug().
			Interface("response_id", response.ID).
			Interface("response_result", response.Result).
			Interface("response_error", response.Error).
			Str("raw_line", string(line)).
			Msg("Received JSON-RPC response from server")

		// Route to the appropriate response channel
		t.requestsMu.Lock()
		log.Debug().
			Interface("response_id", response.ID).
			Interface("pending_ids", getPendingIDs(t.pendingRequests)).
			Msg("Checking pending requests")

		// Normalize response ID (JSON unmarshals numbers as float64)
		responseID := normalizeID(response.ID)

		// Try to find the response channel by checking both the original ID and normalized ID
		responseChan, exists := t.pendingRequests[responseID]
		if !exists {
			// Try with original ID in case it matches
			responseChan, exists = t.pendingRequests[response.ID]
		}

		if exists {
			responseChan <- &response
		} else {
			log.Warn().
				Interface("id", response.ID).
				Interface("normalized_id", responseID).
				Interface("pending_ids", getPendingIDs(t.pendingRequests)).
				Msg("Received response for unknown request ID")
		}
		t.requestsMu.Unlock()
	}
}

// readStderr reads stderr output for logging
func (t *STDIOTransport) readStderr() {
	scanner := bufio.NewScanner(t.stderr)
	for scanner.Scan() {
		log.Debug().Str("stderr", scanner.Text()).Msg("MCP server stderr")
	}
}

// getPendingIDs returns a list of pending request IDs for debugging
func getPendingIDs(pending map[interface{}]chan *JSONRPCResponse) []interface{} {
	ids := make([]interface{}, 0, len(pending))
	for id := range pending {
		ids = append(ids, id)
	}
	return ids
}

// normalizeID converts float64 IDs to int for proper comparison
// JSON unmarshaling converts numbers to float64 by default
func normalizeID(id interface{}) interface{} {
	if f, ok := id.(float64); ok {
		// Convert float64 to int if it's a whole number
		if f == float64(int(f)) {
			return int(f)
		}
	}
	return id
}
