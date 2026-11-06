package mcp

import (
	"context"
	"encoding/json"
)

// Transport defines the interface for MCP communication
type Transport interface {
	// Connect establishes the connection to the MCP server
	Connect(ctx context.Context) error

	// SendRequest sends a JSON-RPC request and returns the response
	SendRequest(ctx context.Context, request JSONRPCRequest) (*JSONRPCResponse, error)

	// SendNotification sends a JSON-RPC notification (no response expected)
	SendNotification(ctx context.Context, notification JSONRPCRequest) error

	// Close closes the connection
	Close() error

	// IsConnected returns true if the transport is connected
	IsConnected() bool
}

// parseJSONRPCResponse parses a JSON-RPC response from bytes
func parseJSONRPCResponse(data []byte) (*JSONRPCResponse, error) {
	var response JSONRPCResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

