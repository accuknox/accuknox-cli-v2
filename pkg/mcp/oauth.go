package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// OAuthHandler manages OAuth 2.1 authorization flow for MCP servers
type OAuthHandler struct {
	mcpServerURL       string
	authServerURL      string
	authServerMetadata *AuthServerMetadata
	clientID           string
	clientSecret       string
	redirectURI        string
	accessToken        string
	refreshToken       string
	tokenExpiry        time.Time
	codeVerifier       string
	codeChallenge      string
	httpClient         *http.Client
	callbackServer     *http.Server
	callbackChan       chan string
}

// OAuthConfig contains OAuth configuration options
type OAuthConfig struct {
	MCPServerURL string
	RedirectURI  string
	Timeout      time.Duration
}

// AuthServerMetadata represents OAuth 2.0 Authorization Server Metadata (RFC 8414)
type AuthServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

// ProtectedResourceMetadata represents OAuth 2.0 Protected Resource Metadata (RFC 9728)
type ProtectedResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
}

// ClientRegistrationRequest represents Dynamic Client Registration request (RFC 7591)
type ClientRegistrationRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

// ClientRegistrationResponse represents Dynamic Client Registration response
type ClientRegistrationResponse struct {
	ClientID         string `json:"client_id"`
	ClientSecret     string `json:"client_secret,omitempty"`
	ClientIDIssuedAt int64  `json:"client_id_issued_at,omitempty"`
}

// TokenResponse represents OAuth token endpoint response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(config *OAuthConfig) *OAuthHandler {
	if config.RedirectURI == "" {
		config.RedirectURI = "http://localhost:8080/callback"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &OAuthHandler{
		mcpServerURL: config.MCPServerURL,
		redirectURI:  config.RedirectURI,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		callbackChan: make(chan string, 1),
	}
}

// Authorize performs the complete OAuth 2.1 authorization flow
func (o *OAuthHandler) Authorize(ctx context.Context) error {
	log.Info().Msg("Starting OAuth 2.1 authorization flow")

	// Step 1: Discover authorization server
	if err := o.discoverAuthServer(ctx); err != nil {
		return fmt.Errorf("authorization server discovery failed: %w", err)
	}

	// Step 2: Register client dynamically (optional - some servers don't support it)
	if err := o.registerClient(ctx); err != nil {
		log.Warn().Err(err).Msg("Dynamic client registration not supported")

		// Provide helpful error message
		return fmt.Errorf(`client registration failed: %w

The authorization server does not support dynamic client registration.
This means you need to pre-register your application with the OAuth provider.

For GitHub MCP Server:
  1. Use a GitHub Personal Access Token instead:
     --token ghp_YOUR_GITHUB_TOKEN
  
  2. Or configure pre-registered client credentials (not yet implemented)

For more information, see: https://docs.github.com/en/authentication`, err)
	}

	// Step 3: Generate PKCE challenge
	o.generatePKCE()

	// Step 4: Start callback server
	if err := o.startCallbackServer(); err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}
	defer o.stopCallbackServer()

	// Step 5: Display authorization URL for user to open manually
	authURL := o.buildAuthorizationURL()
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("🔐 OAuth Authorization Required")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("\nPlease open the following URL in your browser to authorize:")
	fmt.Printf("\n%s\n\n", authURL)
	fmt.Println("After authorization, you will be redirected back to this application.")
	fmt.Println("Waiting for authorization...")
	fmt.Println(strings.Repeat("-", 80))

	// Step 6: Wait for authorization code
	var authCode string
	select {
	case authCode = <-o.callbackChan:
		log.Info().Msg("Received authorization code")
	case <-ctx.Done():
		return fmt.Errorf("authorization cancelled: %w", ctx.Err())
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("authorization timeout")
	}

	// Step 7: Exchange authorization code for tokens
	if err := o.exchangeCodeForTokens(ctx, authCode); err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	log.Info().Msg("OAuth authorization successful")
	return nil
}

// GetAccessToken returns the current access token, refreshing if necessary
func (o *OAuthHandler) GetAccessToken(ctx context.Context) (string, error) {
	// Check if token is expired or about to expire (within 5 minutes)
	if time.Now().Add(5 * time.Minute).After(o.tokenExpiry) {
		if o.refreshToken != "" {
			log.Info().Msg("Access token expired, refreshing...")
			if err := o.refreshAccessToken(ctx); err != nil {
				return "", fmt.Errorf("failed to refresh token: %w", err)
			}
		} else {
			return "", fmt.Errorf("access token expired and no refresh token available")
		}
	}

	return o.accessToken, nil
}

// discoverAuthServer discovers the authorization server using RFC 9728
func (o *OAuthHandler) discoverAuthServer(ctx context.Context) error {
	log.Info().Msg("Discovering authorization server")

	// Try to access MCP server without token to get WWW-Authenticate header
	req, err := http.NewRequestWithContext(ctx, "GET", o.mcpServerURL, nil)
	if err != nil {
		return err
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Should get 401 with WWW-Authenticate header
	if resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("expected 401 Unauthorized, got %d", resp.StatusCode)
	}

	wwwAuth := resp.Header.Get("WWW-Authenticate")
	if wwwAuth == "" {
		return fmt.Errorf("no WWW-Authenticate header found")
	}

	log.Info().Str("www_authenticate", wwwAuth).Msg("Received WWW-Authenticate header")

	// First, try to extract resource_metadata URL (used by GitHub)
	resourceMetadataURL := extractParameter(wwwAuth, "resource_metadata")
	if resourceMetadataURL != "" {
		log.Info().Str("resource_metadata_url", resourceMetadataURL).Msg("Found resource metadata URL")
		return o.discoverFromResourceMetadata(ctx, resourceMetadataURL)
	}

	// Fall back to as_uri parameter (direct authorization server URL)
	asURI := extractParameter(wwwAuth, "as_uri")
	if asURI != "" {
		o.authServerURL = asURI
		log.Info().Str("auth_server", o.authServerURL).Msg("Discovered authorization server from as_uri")
		return nil
	}

	// Last resort: Try well-known OAuth endpoints relative to MCP server URL
	log.Info().Msg("WWW-Authenticate header lacks OAuth metadata, trying well-known endpoints")
	return o.discoverFromWellKnown(ctx)
}

// discoverFromResourceMetadata fetches protected resource metadata and extracts auth servers
func (o *OAuthHandler) discoverFromResourceMetadata(ctx context.Context, metadataURL string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", metadataURL, nil)
	if err != nil {
		return err
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch resource metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch resource metadata: status %d", resp.StatusCode)
	}

	var metadata ProtectedResourceMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return fmt.Errorf("failed to parse resource metadata: %w", err)
	}

	if len(metadata.AuthorizationServers) == 0 {
		return fmt.Errorf("no authorization servers found in resource metadata")
	}

	o.authServerURL = metadata.AuthorizationServers[0]
	log.Info().Str("auth_server", o.authServerURL).Msg("Discovered authorization server from resource metadata")

	return nil
}

// discoverFromWellKnown tries standard OAuth discovery endpoints
func (o *OAuthHandler) discoverFromWellKnown(ctx context.Context) error {
	// Parse the MCP server URL to construct well-known endpoints
	parsedURL, err := url.Parse(o.mcpServerURL)
	if err != nil {
		return fmt.Errorf("failed to parse MCP server URL: %w", err)
	}

	// Try common OAuth well-known endpoints
	wellKnownEndpoints := []string{
		// Standard OAuth 2.0 authorization server metadata (RFC 8414)
		fmt.Sprintf("%s://%s/.well-known/oauth-authorization-server", parsedURL.Scheme, parsedURL.Host),
		// OpenID Connect discovery
		fmt.Sprintf("%s://%s/.well-known/openid-configuration", parsedURL.Scheme, parsedURL.Host),
		// MCP-specific OAuth path (inferred from Cursor's behavior with Sentry)
		fmt.Sprintf("%s://%s/oauth", parsedURL.Scheme, parsedURL.Host),
	}

	for _, endpoint := range wellKnownEndpoints {
		log.Debug().Str("endpoint", endpoint).Msg("Trying well-known endpoint")

		req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			continue
		}

		resp, err := o.httpClient.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			// Try to parse as OAuth metadata
			var metadata AuthServerMetadata
			body, _ := io.ReadAll(resp.Body)
			if err := json.Unmarshal(body, &metadata); err == nil && metadata.AuthorizationEndpoint != "" {
				// Extract base URL from authorization endpoint
				authURL, err := url.Parse(metadata.AuthorizationEndpoint)
				if err == nil {
					o.authServerURL = fmt.Sprintf("%s://%s", authURL.Scheme, authURL.Host)
					log.Info().
						Str("auth_server", o.authServerURL).
						Str("discovery_endpoint", endpoint).
						Msg("Discovered authorization server from well-known endpoint")

					// Store the metadata
					o.authServerMetadata = &metadata
					return nil
				}
			}
		}
	}

	return fmt.Errorf("failed to discover OAuth endpoints from WWW-Authenticate header or well-known endpoints")
}

// registerClient performs dynamic client registration (RFC 7591)
func (o *OAuthHandler) registerClient(ctx context.Context) error {
	// First, get authorization server metadata
	metadata, err := o.getAuthServerMetadata(ctx)
	if err != nil {
		return err
	}

	if metadata.RegistrationEndpoint == "" {
		return fmt.Errorf("authorization server does not support dynamic client registration")
	}

	log.Info().Str("endpoint", metadata.RegistrationEndpoint).Msg("Registering client")

	regRequest := ClientRegistrationRequest{
		ClientName:              "knoxctl-mcp-scanner",
		RedirectURIs:            []string{o.redirectURI},
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		TokenEndpointAuthMethod: "none", // Public client (PKCE)
	}

	reqBody, err := json.Marshal(regRequest)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", metadata.RegistrationEndpoint, strings.NewReader(string(reqBody)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	var regResponse ClientRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResponse); err != nil {
		return err
	}

	o.clientID = regResponse.ClientID
	o.clientSecret = regResponse.ClientSecret

	log.Info().Str("client_id", o.clientID).Msg("Client registered successfully")
	return nil
}

// generatePKCE generates PKCE code verifier and challenge
func (o *OAuthHandler) generatePKCE() {
	// Generate random code verifier (43-128 characters)
	verifier := make([]byte, 32)
	rand.Read(verifier)
	o.codeVerifier = base64.RawURLEncoding.EncodeToString(verifier)

	// Generate code challenge (SHA256 hash of verifier)
	hash := sha256.Sum256([]byte(o.codeVerifier))
	o.codeChallenge = base64.RawURLEncoding.EncodeToString(hash[:])

	log.Debug().Msg("Generated PKCE challenge")
}

// buildAuthorizationURL builds the authorization URL with all required parameters
func (o *OAuthHandler) buildAuthorizationURL() string {
	metadata, _ := o.getAuthServerMetadata(context.Background())

	params := url.Values{}
	params.Set("client_id", o.clientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", o.redirectURI)
	params.Set("code_challenge", o.codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("resource", o.mcpServerURL) // RFC 8707 - Resource Indicators
	params.Set("scope", "openid mcp.access")
	params.Set("state", generateRandomState())

	return metadata.AuthorizationEndpoint + "?" + params.Encode()
}

// exchangeCodeForTokens exchanges the authorization code for access and refresh tokens
func (o *OAuthHandler) exchangeCodeForTokens(ctx context.Context, authCode string) error {
	metadata, err := o.getAuthServerMetadata(ctx)
	if err != nil {
		return err
	}

	log.Info().Msg("Exchanging authorization code for tokens")

	params := url.Values{}
	params.Set("grant_type", "authorization_code")
	params.Set("code", authCode)
	params.Set("redirect_uri", o.redirectURI)
	params.Set("client_id", o.clientID)
	params.Set("code_verifier", o.codeVerifier)

	// Add client_secret if we have one (some servers require it even with PKCE)
	if o.clientSecret != "" {
		params.Set("client_secret", o.clientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", metadata.TokenEndpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Accept any 2xx status code (200 OK, 201 Created, etc.)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return err
	}

	o.accessToken = tokenResp.AccessToken
	o.refreshToken = tokenResp.RefreshToken
	o.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	log.Info().Msg("Tokens obtained successfully")
	return nil
}

// refreshAccessToken refreshes the access token using the refresh token
func (o *OAuthHandler) refreshAccessToken(ctx context.Context) error {
	metadata, err := o.getAuthServerMetadata(ctx)
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Set("grant_type", "refresh_token")
	params.Set("refresh_token", o.refreshToken)
	params.Set("client_id", o.clientID)

	// Add client_secret if we have one
	if o.clientSecret != "" {
		params.Set("client_secret", o.clientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", metadata.TokenEndpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Accept any 2xx status code (200 OK, 201 Created, etc.)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return err
	}

	o.accessToken = tokenResp.AccessToken
	o.refreshToken = tokenResp.RefreshToken
	o.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return nil
}

// getAuthServerMetadata fetches OAuth 2.0 Authorization Server Metadata (RFC 8414)
func (o *OAuthHandler) getAuthServerMetadata(ctx context.Context) (*AuthServerMetadata, error) {
	// If we already have metadata from well-known discovery, use it
	if o.authServerMetadata != nil {
		return o.authServerMetadata, nil
	}

	metadataURL := o.authServerURL + "/.well-known/oauth-authorization-server"

	req, err := http.NewRequestWithContext(ctx, "GET", metadataURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch auth server metadata: status %d", resp.StatusCode)
	}

	var metadata AuthServerMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, err
	}

	o.authServerMetadata = &metadata
	return &metadata, nil
}

// startCallbackServer starts a local HTTP server to receive the OAuth callback
func (o *OAuthHandler) startCallbackServer() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", o.handleCallback)

	// Parse redirect URI to get port
	u, err := url.Parse(o.redirectURI)
	if err != nil {
		return err
	}

	o.callbackServer = &http.Server{
		Addr:    u.Host,
		Handler: mux,
	}

	go func() {
		if err := o.callbackServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Callback server error")
		}
	}()

	return nil
}

// stopCallbackServer stops the callback server
func (o *OAuthHandler) stopCallbackServer() {
	if o.callbackServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		o.callbackServer.Shutdown(ctx)
	}
}

// handleCallback handles the OAuth callback
func (o *OAuthHandler) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		errorMsg := r.URL.Query().Get("error")
		errorDesc := r.URL.Query().Get("error_description")
		http.Error(w, fmt.Sprintf("Authorization failed: %s - %s", errorMsg, errorDesc), http.StatusBadRequest)
		return
	}

	// Send code to channel
	o.callbackChan <- code

	// Send success response
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
		<html>
			<body>
				<h1>Authorization Successful!</h1>
				<p>You can close this window and return to the terminal.</p>
				<script>window.close();</script>
			</body>
		</html>
	`))
}

// Helper functions

// extractParameter extracts a parameter value from WWW-Authenticate header
// Supports both formats: param="value" and param=value
func extractParameter(wwwAuth, param string) string {
	// Parse WWW-Authenticate header
	// Format: Bearer realm="mcp", param="value", other_param="other_value"
	parts := strings.Split(wwwAuth, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		prefix := param + "="
		if strings.Contains(part, prefix) {
			// Extract the value after param=
			idx := strings.Index(part, prefix)
			if idx == -1 {
				continue
			}
			value := part[idx+len(prefix):]
			value = strings.Trim(value, `"`)
			value = strings.TrimSpace(value)
			return value
		}
	}
	return ""
}

func generateRandomState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
