package webrca

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/redhat-appstudio/dora-metrics/pkg/logger"
)

// Client handles HTTP communication with the WebRCA API.
// It manages authentication, token refresh, and incident data retrieval.
type Client struct {
	httpClient   *http.Client
	baseURL      string
	offlineToken string
	accessToken  string
	tokenExpiry  time.Time
	mu           sync.RWMutex // Protects token access
}

// NewClient creates a new WebRCA API client with proper configuration.
// It initializes the HTTP client with appropriate timeouts and sets up
// authentication using the provided offline token.
func NewClient(baseURL, offlineToken string) *Client {
	if baseURL == "" {
		baseURL = DefaultWebRCAAPIURL
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: DefaultHTTPTimeout,
		},
		baseURL:      baseURL,
		offlineToken: offlineToken,
	}
}

// TokenResponse represents the response from the OAuth token endpoint.
// It contains the access token and metadata needed for API authentication.
type TokenResponse struct {
	// AccessToken is the short-lived access token for API requests
	AccessToken string `json:"access_token"`

	// TokenType is the type of token (typically "Bearer")
	TokenType string `json:"token_type"`

	// ExpiresIn is the number of seconds until the token expires
	ExpiresIn int `json:"expires_in"`
}

// getAccessToken retrieves a valid access token for API authentication.
// It first checks if the cached token is still valid, and if not,
// requests a new token using the offline token.
func (c *Client) getAccessToken() (string, error) {
	// Fast path: check if token is valid with read lock
	c.mu.RLock()
	if c.isTokenValid() {
		token := c.accessToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	// Slow path: acquire write lock and refresh token
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.isTokenValid() {
		return c.accessToken, nil
	}

	// Request new token using offline token
	token, err := c.requestNewToken()
	if err != nil {
		return "", fmt.Errorf("%s: %w", ErrTokenRequest, err)
	}

	// Cache the token
	c.cacheToken(token)
	return c.accessToken, nil
}

// isTokenValid checks if the cached token is still valid
func (c *Client) isTokenValid() bool {
	return c.accessToken != "" && time.Now().Before(c.tokenExpiry)
}

// requestNewToken makes a request to get a new access token
func (c *Client) requestNewToken() (*TokenResponse, error) {
	// Pre-allocate and build form data efficiently
	data := url.Values{
		"grant_type":    {OAuth2GrantType},
		"client_id":     {OAuth2ClientID},
		"refresh_token": {c.offlineToken},
	}

	req, err := http.NewRequest("POST", OAuth2TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", OAuth2ContentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrHTTPRequest, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != HTTPStatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrTokenDecode, err)
	}

	if tokenResp.AccessToken == "" {
		return nil, errors.New(ErrTokenEmpty)
	}

	return &tokenResp, nil
}

// cacheToken caches the access token with proper expiry
func (c *Client) cacheToken(token *TokenResponse) {
	c.accessToken = token.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(token.ExpiresIn)*time.Second - TokenRefreshBuffer)
}

// GetAllIncidents fetches all incidents from the WebRCA API with automatic pagination.
// It handles authentication, pagination, and returns a complete list of all incidents.
// Returns an error if authentication fails or API requests fail.
func (c *Client) GetAllIncidents(ctx context.Context) ([]Incident, error) {
	// Get access token (cached or fresh)
	token, err := c.getAccessToken()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrTokenRequest, err)
	}

	return c.fetchAllPages(ctx, token)
}

// fetchAllPages handles pagination logic for fetching all incidents from the API.
// It iterates through all pages until no more incidents are available.
func (c *Client) fetchAllPages(ctx context.Context, token string) ([]Incident, error) {
	// Pre-allocate slice with reasonable capacity to reduce allocations
	allIncidents := make([]Incident, 0, DefaultPageSize*2) // Start with 2 pages worth
	page := 1

	for {
		incidentList, err := c.fetchPage(ctx, token, page, DefaultPageSize)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ErrIncidentFetch, err)
		}

		// Use append with pre-allocated capacity
		allIncidents = append(allIncidents, incidentList.Items...)
		logger.Debugf("Fetched page %d (%d items)", page, len(incidentList.Items))

		// Stop if we got fewer items than requested (last page)
		if len(incidentList.Items) < DefaultPageSize {
			break
		}

		page++
	}

	return allIncidents, nil
}

// fetchPage fetches a single page of incidents from the WebRCA API.
// It makes an authenticated request and returns the paginated response.
func (c *Client) fetchPage(ctx context.Context, token string, page, size int) (*IncidentList, error) {
	// Use strings.Builder for efficient URL construction
	var urlBuilder strings.Builder
	urlBuilder.Grow(len(c.baseURL) + 50) // Pre-allocate reasonable capacity
	urlBuilder.WriteString(c.baseURL)
	urlBuilder.WriteString("?page=")
	urlBuilder.WriteString(fmt.Sprintf("%d", page))
	urlBuilder.WriteString("&size=")
	urlBuilder.WriteString(fmt.Sprintf("%d", size))
	url := urlBuilder.String()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrHTTPRequest, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != HTTPStatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var incidentList IncidentList
	if err := json.Unmarshal(body, &incidentList); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrIncidentParse, err)
	}

	return &incidentList, nil
}
