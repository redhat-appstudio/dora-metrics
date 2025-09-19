package webrca

import "time"

// OAuth2 configuration constants
const (
	// OAuth2TokenURL is the Red Hat SSO token endpoint
	OAuth2TokenURL = "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token"

	// OAuth2ClientID is the client ID for Red Hat cloud services
	OAuth2ClientID = "cloud-services"

	// OAuth2GrantType is the grant type for refresh token flow
	OAuth2GrantType = "refresh_token"

	// OAuth2ContentType is the content type for token requests
	OAuth2ContentType = "application/x-www-form-urlencoded"
)

// HTTP configuration constants
const (
	// DefaultHTTPTimeout is the default timeout for HTTP requests
	DefaultHTTPTimeout = 120 * time.Second

	// DefaultPageSize is the default number of items per page for pagination
	DefaultPageSize = 100

	// TokenRefreshBuffer is the time buffer before token expiry to refresh
	TokenRefreshBuffer = 60 * time.Second
)

// API configuration constants
const (
	// DefaultWebRCAAPIURL is the default WebRCA incidents API endpoint
	DefaultWebRCAAPIURL = "https://api.openshift.com/api/web-rca/v1/incidents"

	// DefaultCheckInterval is the default interval for incident checks
	DefaultCheckInterval = 30 * time.Minute
)

// HTTP status codes
const (
	HTTPStatusOK = 200
)

// Error messages
const (
	ErrMissingConfig = "missing required configuration"
	ErrTokenRequest  = "failed to get access token"
	ErrTokenEmpty    = "access token is empty"
	ErrHTTPRequest   = "HTTP request failed"
	ErrTokenDecode   = "failed to decode token response"
	ErrIncidentFetch = "failed to fetch incidents"
	ErrIncidentParse = "failed to parse incident data"
)
