package mcp

// ScanOptions contains configuration for MCP scanning
type ScanOptions struct {
	// Transport options
	HTTPUrl   string   // URL of the MCP server (for HTTP transport)
	SSEUrl    string   // URL of the MCP server (for SSE transport, e.g., Atlassian)
	STDIOCmd  string   // Command to run for STDIO transport
	STDIOArgs []string // Arguments for STDIO command
	Timeout   int      // Request timeout in seconds (default: 120)

	// OAuth options
	UseOAuth       bool   // Enable OAuth 2.1 authorization
	OAuthToken     string // Pre-existing OAuth token (skip authorization flow)
	SaveTokens     bool   // Save OAuth tokens for reuse
	TokenCachePath string // Path to save/load tokens

	// Scanning options
	ScanAPI           string // URL of the hosted scanning API (default: http://localhost:5001/analyze)
	ScanAPIToken      string // Bearer token for the scanning API
	ScanToolsOnly     bool   // Only scan tools
	ScanPromptsOnly   bool   // Only scan prompts
	ScanResourcesOnly bool   // Only scan resources

	// Output options
	OutputFormat string // Output format: text, json (default: text)
	LogLevel     string // Log level: debug, info, warn, error, fatal, panic, disabled (default: info)
	Verbose      bool   // Enable verbose logging (deprecated, use LogLevel)
	Debug        bool   // Enable debug logging (deprecated, use LogLevel)
}

// Core result types for display
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type PromptInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ResourceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	URI         string `json:"uri"`
}

// JSON-RPC 2.0 message types (simplified)
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"` // omitempty for notifications
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP Initialize types (simplified)
type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation     `json:"clientInfo"`
}

type ClientCapabilities struct {
	// Minimal capabilities - only what we need
}

type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCP response types for listing (simplified)
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"inputSchema,omitempty"` // We don't need to parse this
}

type ListPromptsResult struct {
	Prompts []Prompt `json:"prompts"`
}

type Prompt struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Arguments   interface{} `json:"arguments,omitempty"` // We don't need to parse this
}

type ListResourcesResult struct {
	Resources []Resource `json:"resources"`
}

type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

// ScanResult represents the complete scan results for JSON output
type ScanResult struct {
	Timestamp    string           `json:"timestamp"`
	MCPServerURL string           `json:"mcp_server_url"`
	Tools        []ItemScanResult `json:"tools,omitempty"`
	Prompts      []ItemScanResult `json:"prompts,omitempty"`
	Resources    []ItemScanResult `json:"resources,omitempty"`
	Summary      ScanSummary      `json:"summary"`
}

// ItemScanResult represents scan results for a single tool/prompt/resource
type ItemScanResult struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"` // "tool", "prompt", or "resource"
	URI         string                 `json:"uri,omitempty"`
	Analysis    map[string]interface{} `json:"analysis"`
}

// ScanSummary provides a summary of scan results
type ScanSummary struct {
	TotalScanned         int `json:"total_scanned"`
	VulnerabilitiesFound int `json:"vulnerabilities_found"`
	PromptInjections     int `json:"prompt_injections"`
	SecretsExposed       int `json:"secrets_exposed"`
	MaliciousCode        int `json:"malicious_code"`
}
