package cmd

import (
	"fmt"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/mcp"
	"github.com/spf13/cobra"
)

var mcpScanOpts mcp.ScanOptions

var mcpScanCmd = &cobra.Command{
	Use:   "mcp-scan",
	Short: "Scan MCP servers for security vulnerabilities",
	Long: `
Connect to an MCP (Model Context Protocol) server and analyze tools, prompts, and resources 
for potential security vulnerabilities including:
  - Prompt injection attacks in descriptions
  - Exposed secrets and API keys
  - Malicious code patterns
  - PII exposure

This command supports both HTTP and STDIO transports, and can handle OAuth 2.1 
authorization for protected MCP servers.

Examples:
  # Scan HTTP MCP server
  knoxctl mcp-scan --http-url https://mcp-server.example.com

  # Scan STDIO MCP server
  knoxctl mcp-scan --stdio "python mcp-server.py"

  # Scan with OAuth authorization
  knoxctl mcp-scan --http-url https://mcp-server.example.com --oauth

  # Output results as JSON
  knoxctl mcp-scan --http-url https://mcp-server.example.com --output json

  # Scan only tools
  knoxctl mcp-scan --http-url https://mcp-server.example.com --tools-only
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate transport options
		if mcpScanOpts.HTTPUrl == "" && mcpScanOpts.SSEUrl == "" && mcpScanOpts.STDIOCmd == "" {
			return fmt.Errorf("either --http-url, --sse-url, or --stdio flag is required")
		}

		// Check for conflicting transport options
		transportCount := 0
		if mcpScanOpts.HTTPUrl != "" {
			transportCount++
		}
		if mcpScanOpts.SSEUrl != "" {
			transportCount++
		}
		if mcpScanOpts.STDIOCmd != "" {
			transportCount++
		}
		if transportCount > 1 {
			return fmt.Errorf("cannot specify multiple transport options (--http-url, --sse-url, --stdio)")
		}

		// Parse STDIO args if provided
		if mcpScanOpts.STDIOCmd != "" {
			parts := strings.Fields(mcpScanOpts.STDIOCmd)
			if len(parts) > 1 {
				mcpScanOpts.STDIOCmd = parts[0]
				mcpScanOpts.STDIOArgs = parts[1:]
			}
		}

		// Create and run scanner
		scanner := mcp.New(&mcpScanOpts)

		if err := scanner.Scan(); err != nil {
			fmt.Printf("Error scanning MCP server: %v\n", err)
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(mcpScanCmd)

	// Transport options
	mcpScanCmd.Flags().StringVar(&mcpScanOpts.HTTPUrl, "http-url", "", "URL of the MCP server exposed via HTTP")
	mcpScanCmd.Flags().StringVar(&mcpScanOpts.SSEUrl, "sse-url", "", "URL of the MCP server using SSE transport (e.g., Atlassian)")
	mcpScanCmd.Flags().StringVar(&mcpScanOpts.STDIOCmd, "stdio", "", "Command to run for STDIO MCP server (e.g., 'python server.py')")
	mcpScanCmd.Flags().IntVar(&mcpScanOpts.Timeout, "timeout", 120, "Request timeout in seconds")

	// OAuth options
	mcpScanCmd.Flags().BoolVar(&mcpScanOpts.UseOAuth, "oauth", false, "Enable OAuth 2.1 authorization")
	mcpScanCmd.Flags().StringVar(&mcpScanOpts.OAuthToken, "token", "", "Use existing OAuth bearer token (skip authorization flow)")
	mcpScanCmd.Flags().BoolVar(&mcpScanOpts.SaveTokens, "save-tokens", false, "Save OAuth tokens for reuse")
	mcpScanCmd.Flags().StringVar(&mcpScanOpts.TokenCachePath, "token-cache", "", "Path to save/load OAuth tokens")

	// Scanning options
	mcpScanCmd.Flags().StringVar(&mcpScanOpts.ScanAPI, "scanner-api", "http://34.46.3.177:8000/mcp_query", "URL of the hosted scanning API")
	mcpScanCmd.Flags().StringVar(&mcpScanOpts.ScanAPIToken, "scanner-api-token", "", "Bearer token for the scanning API")
	mcpScanCmd.Flags().BoolVar(&mcpScanOpts.ScanToolsOnly, "tools-only", false, "Only scan tools (skip prompts and resources)")
	mcpScanCmd.Flags().BoolVar(&mcpScanOpts.ScanPromptsOnly, "prompts-only", false, "Only scan prompts (skip tools and resources)")
	mcpScanCmd.Flags().BoolVar(&mcpScanOpts.ScanResourcesOnly, "resources-only", false, "Only scan resources (skip tools and prompts)")

	// Output options
	mcpScanCmd.Flags().StringVar(&mcpScanOpts.OutputFormat, "output", "text", "Output format: text, json")
	mcpScanCmd.Flags().StringVar(&mcpScanOpts.LogLevel, "log-level", "warn", "Log level: debug, info, warn, error, fatal, panic, disabled (default: warn)")
	mcpScanCmd.Flags().BoolVar(&mcpScanOpts.Verbose, "verbose", false, "Enable verbose logging (deprecated, use --log-level info)")
	mcpScanCmd.Flags().BoolVar(&mcpScanOpts.Debug, "debug", false, "Enable debug logging (deprecated, use --log-level debug)")
}
