package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Current MCP protocol version (2025-06-18)
const MCPProtocolVersion = "2025-06-18"

// Scanner handles MCP server scanning operations
type Scanner struct {
	options      *ScanOptions
	transport    Transport
	oauthHandler *OAuthHandler
	httpClient   *http.Client
	requestID    int
	scanResults  *ScanResult
}

// New creates a new MCP scanner with the given options
func New(options *ScanOptions) *Scanner {
	// Set defaults
	if options.Timeout == 0 {
		options.Timeout = 30
	}
	if options.ScanAPI == "" {
		options.ScanAPI = "http://34.46.3.177:8000/mcp_query"
	}
	if options.OutputFormat == "" {
		options.OutputFormat = "text"
	}
	if options.LogLevel == "" {
		// Default to 'warn' for cleaner output
		options.LogLevel = "warn"

		// Legacy flags override default
		if options.Debug {
			options.LogLevel = "debug"
		} else if options.Verbose {
			options.LogLevel = "info"
		}
	}

	// Configure global log level
	configureLogLevel(options.LogLevel)

	return &Scanner{
		options: options,
		httpClient: &http.Client{
			Timeout: time.Duration(options.Timeout) * time.Second,
		},
		requestID: 1,
		scanResults: &ScanResult{
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}
}

// configureLogLevel sets the global zerolog level
func configureLogLevel(level string) {
	// Configure pretty console output
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Set log level based on flag
	switch strings.ToLower(level) {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn", "warning":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "fatal":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case "panic":
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	case "disabled", "off", "none":
		zerolog.SetGlobalLevel(zerolog.Disabled)
	default:
		// Default to warn for cleaner output
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	}
}

// Scan connects to the MCP server and retrieves tools, prompts, and resources
func (s *Scanner) Scan() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.options.Timeout)*time.Second)
	defer cancel()

	// Step 1: Setup transport
	if err := s.setupTransport(ctx); err != nil {
		return fmt.Errorf("failed to setup transport: %w", err)
	}
	defer s.transport.Close()

	// Step 2: Initialize MCP connection
	if err := s.initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize MCP connection: %w", err)
	}

	// Step 3: Scan components based on options
	s.scanComponents(ctx)

	// Step 4: Calculate summary
	s.calculateSummary()

	// Step 5: Display results
	s.displayResultsFormatted()

	return nil
}

// setupTransport creates and configures the appropriate transport
func (s *Scanner) setupTransport(ctx context.Context) error {
	timeout := time.Duration(s.options.Timeout) * time.Second

	// Determine transport type
	if s.options.SSEUrl != "" {
		// SSE transport (used by Atlassian)
		log.Info().Str("url", s.options.SSEUrl).Msg("Using SSE transport")
		sseTransport := NewSSETransport(s.options.SSEUrl, timeout)
		s.transport = sseTransport
		s.scanResults.MCPServerURL = s.options.SSEUrl

		// Handle OAuth if required
		if s.options.UseOAuth || s.options.OAuthToken != "" {
			if s.options.OAuthToken != "" {
				// Use provided token
				sseTransport.SetOAuthToken(s.options.OAuthToken)
			} else {
				// Perform OAuth flow
				s.oauthHandler = NewOAuthHandler(&OAuthConfig{
					MCPServerURL: s.options.SSEUrl,
					Timeout:      timeout,
				})

				if err := s.oauthHandler.Authorize(ctx); err != nil {
					return fmt.Errorf("OAuth authorization failed: %w", err)
				}

				token, err := s.oauthHandler.GetAccessToken(ctx)
				if err != nil {
					return fmt.Errorf("failed to get access token: %w", err)
				}

				sseTransport.SetOAuthToken(token)
			}
		}

	} else if s.options.HTTPUrl != "" {
		// Regular HTTP/Streamable HTTP transport
		httpTransport := NewHTTPTransport(s.options.HTTPUrl, timeout)
		s.transport = httpTransport
		s.scanResults.MCPServerURL = s.options.HTTPUrl

		// Handle OAuth if required
		if s.options.UseOAuth || s.options.OAuthToken != "" {
			if s.options.OAuthToken != "" {
				// Use provided token
				httpTransport.SetOAuthToken(s.options.OAuthToken)
			} else {
				// Perform OAuth flow
				s.oauthHandler = NewOAuthHandler(&OAuthConfig{
					MCPServerURL: s.options.HTTPUrl,
					Timeout:      timeout,
				})

				if err := s.oauthHandler.Authorize(ctx); err != nil {
					return fmt.Errorf("OAuth authorization failed: %w", err)
				}

				token, err := s.oauthHandler.GetAccessToken(ctx)
				if err != nil {
					return fmt.Errorf("failed to get access token: %w", err)
				}

				httpTransport.SetOAuthToken(token)
			}
		}

	} else if s.options.STDIOCmd != "" {
		// STDIO transport
		stdioTransport := NewSTDIOTransport(s.options.STDIOCmd, s.options.STDIOArgs...)
		s.transport = stdioTransport
		s.scanResults.MCPServerURL = fmt.Sprintf("stdio://%s", s.options.STDIOCmd)

	} else {
		return fmt.Errorf("no transport specified (provide --http-url, --sse-url, or --stdio)")
	}

	// Connect the transport
	return s.transport.Connect(ctx)
}

// initialize performs MCP protocol initialization
func (s *Scanner) initialize(ctx context.Context) error {
	initParams := InitializeParams{
		ProtocolVersion: MCPProtocolVersion,
		Capabilities:    ClientCapabilities{},
		ClientInfo: Implementation{
			Name:    "knoxctl-mcp-scanner",
			Version: "1.0.0",
		},
	}

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      s.nextRequestID(),
		Method:  "initialize",
		Params:  initParams,
	}

	response, err := s.transport.SendRequest(ctx, request)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	if response.Error != nil {
		return fmt.Errorf("initialize error: %s (code: %d)", response.Error.Message, response.Error.Code)
	}

	// Send initialized notification
	notification := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}

	if err := s.transport.SendNotification(ctx, notification); err != nil {
		log.Warn().Err(err).Msg("Failed to send initialized notification")
	}

	return nil
}

// scanComponents scans the requested MCP components
func (s *Scanner) scanComponents(ctx context.Context) {
	// Scan tools (unless explicitly excluded)
	if !s.options.ScanPromptsOnly && !s.options.ScanResourcesOnly {
		tools := s.listTools(ctx)
		for _, tool := range tools {
			analysis := s.analyzeForInjection(tool.Name, tool.Description, "tool")
			s.scanResults.Tools = append(s.scanResults.Tools, ItemScanResult{
				Name:        tool.Name,
				Description: tool.Description,
				Type:        "tool",
				Analysis:    analysis,
			})
		}
	}

	// Scan prompts (unless explicitly excluded)
	if !s.options.ScanToolsOnly && !s.options.ScanResourcesOnly {
		prompts := s.listPrompts(ctx)
		for _, prompt := range prompts {
			analysis := s.analyzeForInjection(prompt.Name, prompt.Description, "prompt")
			s.scanResults.Prompts = append(s.scanResults.Prompts, ItemScanResult{
				Name:        prompt.Name,
				Description: prompt.Description,
				Type:        "prompt",
				Analysis:    analysis,
			})
		}
	}

	// Scan resources (unless explicitly excluded)
	if !s.options.ScanToolsOnly && !s.options.ScanPromptsOnly {
		resources := s.listResources(ctx)
		for _, resource := range resources {
			analysis := s.analyzeForInjection(resource.Name, resource.Description, "resource")
			s.scanResults.Resources = append(s.scanResults.Resources, ItemScanResult{
				Name:        resource.Name,
				Description: resource.Description,
				Type:        "resource",
				URI:         resource.URI,
				Analysis:    analysis,
			})
		}
	}
}

// listTools retrieves all available tools from the MCP server
func (s *Scanner) listTools(ctx context.Context) []ToolInfo {
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      s.nextRequestID(),
		Method:  "tools/list",
	}

	response, err := s.transport.SendRequest(ctx, request)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list tools")
		return []ToolInfo{}
	}

	if response.Error != nil {
		log.Warn().Str("error", response.Error.Message).Msg("Tools list error")
		return []ToolInfo{}
	}

	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to marshal tools result")
		return []ToolInfo{}
	}

	var listResult ListToolsResult
	if err := json.Unmarshal(resultBytes, &listResult); err != nil {
		log.Warn().Err(err).Msg("Failed to unmarshal tools result")
		return []ToolInfo{}
	}

	var tools []ToolInfo
	for _, tool := range listResult.Tools {
		tools = append(tools, ToolInfo{
			Name:        tool.Name,
			Description: tool.Description,
		})
	}

	return tools
}

// listPrompts retrieves all available prompts from the MCP server
func (s *Scanner) listPrompts(ctx context.Context) []PromptInfo {
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      s.nextRequestID(),
		Method:  "prompts/list",
	}

	response, err := s.transport.SendRequest(ctx, request)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list prompts")
		return []PromptInfo{}
	}

	if response.Error != nil {
		log.Warn().Str("error", response.Error.Message).Msg("Prompts list error")
		return []PromptInfo{}
	}

	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to marshal prompts result")
		return []PromptInfo{}
	}

	var listResult ListPromptsResult
	if err := json.Unmarshal(resultBytes, &listResult); err != nil {
		log.Warn().Err(err).Msg("Failed to unmarshal prompts result")
		return []PromptInfo{}
	}

	var prompts []PromptInfo
	for _, prompt := range listResult.Prompts {
		prompts = append(prompts, PromptInfo{
			Name:        prompt.Name,
			Description: prompt.Description,
		})
	}

	return prompts
}

// listResources retrieves all available resources from the MCP server
func (s *Scanner) listResources(ctx context.Context) []ResourceInfo {
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      s.nextRequestID(),
		Method:  "resources/list",
	}

	response, err := s.transport.SendRequest(ctx, request)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list resources")
		return []ResourceInfo{}
	}

	if response.Error != nil {
		log.Warn().Str("error", response.Error.Message).Msg("Resources list error")
		return []ResourceInfo{}
	}

	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to marshal resources result")
		return []ResourceInfo{}
	}

	var listResult ListResourcesResult
	if err := json.Unmarshal(resultBytes, &listResult); err != nil {
		log.Warn().Err(err).Msg("Failed to unmarshal resources result")
		return []ResourceInfo{}
	}

	var resources []ResourceInfo
	for _, resource := range listResult.Resources {
		resources = append(resources, ResourceInfo{
			Name:        resource.Name,
			Description: resource.Description,
			URI:         resource.URI,
		})
	}

	return resources
}

// nextRequestID returns the next request ID
func (s *Scanner) nextRequestID() int {
	id := s.requestID
	s.requestID++
	return id
}

// calculateSummary calculates the scan summary
func (s *Scanner) calculateSummary() {
	summary := &s.scanResults.Summary
	summary.TotalScanned = len(s.scanResults.Tools) + len(s.scanResults.Prompts) + len(s.scanResults.Resources)

	// Analyze all items
	allItems := []ItemScanResult{}
	allItems = append(allItems, s.scanResults.Tools...)
	allItems = append(allItems, s.scanResults.Prompts...)
	allItems = append(allItems, s.scanResults.Resources...)

	for _, item := range allItems {
		itemHasVuln := false

		// Check prompt injection
		if pi, ok := item.Analysis["prompt_injection"].(map[string]interface{}); ok {
			if detected, _ := pi["detected"].(bool); detected {
				summary.PromptInjections++
				itemHasVuln = true
			}
		}

		// Check malicious code
		if code, ok := item.Analysis["code"].(map[string]interface{}); ok {
			if detected, _ := code["detected"].(bool); detected {
				summary.MaliciousCode++
				itemHasVuln = true
			}
		}

		// Check secrets
		if secrets, ok := item.Analysis["secrets"].(map[string]interface{}); ok {
			if detected, _ := secrets["detected"].(bool); detected {
				summary.SecretsExposed++
				itemHasVuln = true
			}
		}

		if itemHasVuln {
			summary.VulnerabilitiesFound++
		}
	}
}

// displayResultsFormatted displays results in the requested format
func (s *Scanner) displayResultsFormatted() {
	if s.options.OutputFormat == "json" {
		s.displayJSON()
	} else {
		s.displayText()
	}
}

// displayJSON outputs results in JSON format
func (s *Scanner) displayJSON() {
	output, err := json.MarshalIndent(s.scanResults, "", "  ")
	if err != nil {
		log.Error().Err(err).Msg("Failed to encode JSON output")
		return
	}
	fmt.Println(string(output))
}

// displayText outputs results in human-readable text format
func (s *Scanner) displayText() {
	fmt.Println("=== MCP SERVER SCAN RESULTS ===")
	fmt.Printf("\nServer: %s\n", s.scanResults.MCPServerURL)
	fmt.Printf("Timestamp: %s\n", s.scanResults.Timestamp)

	if len(s.scanResults.Tools) > 0 {
		fmt.Println("\n📋 TOOLS:")
		for i, item := range s.scanResults.Tools {
			fmt.Printf("\n%d. %s\n", i+1, item.Name)
			fmt.Printf("   Description: %s\n", item.Description)
			displayAnalysisResult(item.Analysis)
		}
	}

	if len(s.scanResults.Prompts) > 0 {
		fmt.Println("\n💬 PROMPTS:")
		for i, item := range s.scanResults.Prompts {
			fmt.Printf("\n%d. %s\n", i+1, item.Name)
			fmt.Printf("   Description: %s\n", item.Description)
			displayAnalysisResult(item.Analysis)
		}
	}

	if len(s.scanResults.Resources) > 0 {
		fmt.Println("\n📁 RESOURCES:")
		for i, item := range s.scanResults.Resources {
			fmt.Printf("\n%d. %s\n", i+1, item.Name)
			fmt.Printf("   URI: %s\n", item.URI)
			fmt.Printf("   Description: %s\n", item.Description)
			displayAnalysisResult(item.Analysis)
		}
	}

	if s.scanResults.Summary.TotalScanned == 0 {
		fmt.Println("\n❌ No tools, prompts, or resources found.")
	}

	// Display summary
	fmt.Println("\n=== SUMMARY ===")
	fmt.Printf("Total scanned: %d items\n", s.scanResults.Summary.TotalScanned)
	fmt.Printf("Vulnerabilities found: %d\n", s.scanResults.Summary.VulnerabilitiesFound)
	fmt.Printf("  - Prompt injections: %d\n", s.scanResults.Summary.PromptInjections)
	fmt.Printf("  - Secrets exposed: %d\n", s.scanResults.Summary.SecretsExposed)
	fmt.Printf("  - Malicious code: %d\n", s.scanResults.Summary.MaliciousCode)

	if s.scanResults.Summary.VulnerabilitiesFound > 0 {
		fmt.Println("\nRecommendation: Review and sanitize flagged items before deployment.")
	}

	fmt.Println("\n=== END SCAN ===")
}

// analyzeForInjection sends metadata to hosted scanning API for analysis
func (s *Scanner) analyzeForInjection(name, description, itemType string) map[string]interface{} {
	// Request format: {"prompt": "description text"}
	requestData := map[string]interface{}{
		"prompt": description,
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(requestData); err != nil {
		return map[string]interface{}{"error": "Failed to marshal request"}
	}

	jsonData := bytes.TrimSpace(buf.Bytes())

	if s.options.Debug {
		log.Debug().Msgf("Sending to scan API: %s", string(jsonData))
	}

	// Create request with Authorization header
	req, err := http.NewRequest("POST", s.options.ScanAPI, bytes.NewBuffer(jsonData))
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Failed to create request: %v", err)}
	}

	req.Header.Set("Content-Type", "application/json")

	// Add bearer token if provided
	if s.options.ScanAPIToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.options.ScanAPIToken))
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("HTTP request failed: %v", err)}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]interface{}{"error": "Failed to read response"}
	}

	if s.options.Debug {
		log.Debug().Msgf("Received from scan API: %s", string(body))
	}

	// Parse the API response
	// Response format: {"response":{"Code":[text,bool,float],"PromptInjection":[...],"Secrets":[...]},"status":"success"}
	// Note: true = passed (safe), false = violating (vulnerable)
	var apiResponse struct {
		Response struct {
			Code            []interface{} `json:"Code"`
			PromptInjection []interface{} `json:"PromptInjection"`
			Secrets         []interface{} `json:"Secrets"`
		} `json:"response"`
		Status string `json:"status"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Failed to parse response: %v", err)}
	}

	// Extract results (true = safe, false = vulnerable)
	promptInjectionPassed := true
	codePassed := true
	secretsPassed := true

	if len(apiResponse.Response.PromptInjection) > 1 {
		if val, ok := apiResponse.Response.PromptInjection[1].(bool); ok {
			promptInjectionPassed = val
		}
	}
	if len(apiResponse.Response.Code) > 1 {
		if val, ok := apiResponse.Response.Code[1].(bool); ok {
			codePassed = val
		}
	}
	if len(apiResponse.Response.Secrets) > 1 {
		if val, ok := apiResponse.Response.Secrets[1].(bool); ok {
			secretsPassed = val
		}
	}

	// Transform to our expected format
	// is_injection: true if NOT passed (false = vulnerability)
	isInjection := !promptInjectionPassed
	isCode := !codePassed
	isSecrets := !secretsPassed

	// Build result in expected format - keep separate detections
	result := map[string]interface{}{
		"prompt_injection": map[string]interface{}{
			"detected": isInjection,
		},
		"code": map[string]interface{}{
			"detected": isCode,
		},
		"secrets": map[string]interface{}{
			"detected": isSecrets,
		},
	}

	return result
}

// displayAnalysisResult displays analysis results for a single item
func displayAnalysisResult(analysis map[string]interface{}) {
	if errorMsg, exists := analysis["error"]; exists {
		fmt.Printf("     ⚠️  Analysis Error: %v\n", errorMsg)
		return
	}

	// Prompt Injection detection
	if pi, ok := analysis["prompt_injection"].(map[string]interface{}); ok {
		if detected, _ := pi["detected"].(bool); detected {
			fmt.Printf("     🚨 PROMPT INJECTION DETECTED\n")
		} else {
			fmt.Printf("     ✅ No prompt injection detected\n")
		}
	}

	// Malicious Code detection
	if code, ok := analysis["code"].(map[string]interface{}); ok {
		if detected, _ := code["detected"].(bool); detected {
			fmt.Printf("     ⚠️  MALICIOUS CODE DETECTED\n")
		} else {
			fmt.Printf("     ✅ No malicious code detected\n")
		}
	}

	// Secrets detection
	if secrets, ok := analysis["secrets"].(map[string]interface{}); ok {
		if detected, _ := secrets["detected"].(bool); detected {
			fmt.Printf("     🔑 SECRETS DETECTED\n")
		} else {
			fmt.Printf("     ✅ No secrets detected\n")
		}
	}
}
