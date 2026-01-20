package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenShiftAPIEndpoint is the OpenShift API endpoint for user info
const OpenShiftAPIEndpoint = "https://api.openshift.com/apis/user.openshift.io/v1/users/~"

// DefaultHTTPTimeout is the default timeout for HTTP requests
const DefaultHTTPTimeout = 10 * time.Second

// Validator handles token validation and email extraction
type Validator struct {
	httpClient *http.Client
}

// NewValidator creates a new token validator
func NewValidator() *Validator {
	return &Validator{
		httpClient: &http.Client{
			Timeout: DefaultHTTPTimeout,
		},
	}
}

// ValidateTokenAndExtractEmail validates a token against OpenShift API and extracts the email
// This is the main function that should be used for authentication
func (v *Validator) ValidateTokenAndExtractEmail(token string) (string, error) {
	// First, try to extract email from JWT token (faster, no API call)
	email, err := v.ExtractEmailFromToken(token)
	if err == nil {
		// If we can extract email, validate token by calling OpenShift API
		if err := v.ValidateTokenWithAPI(token); err != nil {
			return "", fmt.Errorf("token validation failed: %w", err)
		}
		return email, nil
	}

	// If JWT extraction fails, try to get user info from OpenShift API
	return v.GetUserInfoFromAPI(token)
}

// ExtractEmailFromToken extracts email from OpenShift JWT token
func (v *Validator) ExtractEmailFromToken(token string) (string, error) {
	// JWT tokens have 3 parts separated by dots: header.payload.signature
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("invalid token format")
	}

	// Decode the payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode token payload: %w", err)
	}

	// Parse JSON payload
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("failed to parse token claims: %w", err)
	}

	// Extract email from claims (OpenShift tokens use "email" or "preferred_username")
	email, ok := claims["email"].(string)
	if !ok {
		// Try preferred_username as fallback
		email, ok = claims["preferred_username"].(string)
		if !ok {
			return "", errors.New("email not found in token claims")
		}
	}

	return email, nil
}

// ValidateTokenWithAPI validates the token by making a request to OpenShift API
func (v *Validator) ValidateTokenWithAPI(token string) error {
	req, err := http.NewRequest("GET", OpenShiftAPIEndpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return errors.New("token is invalid or expired")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetUserInfoFromAPI gets user information from OpenShift API using the token
func (v *Validator) GetUserInfoFromAPI(token string) (string, error) {
	req, err := http.NewRequest("GET", OpenShiftAPIEndpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", errors.New("token is invalid or expired")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var userInfo map[string]interface{}
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return "", fmt.Errorf("failed to parse user info: %w", err)
	}

	// Extract email from user info
	// OpenShift user info typically has email in metadata or spec
	metadata, ok := userInfo["metadata"].(map[string]interface{})
	if ok {
		if email, ok := metadata["name"].(string); ok && strings.Contains(email, "@") {
			return email, nil
		}
	}

	// Try to extract from JWT as fallback
	return v.ExtractEmailFromToken(token)
}

// ValidateRedHatEmail checks if the email is from Red Hat domain
func ValidateRedHatEmail(email string) bool {
	return strings.HasSuffix(email, "@redhat.com")
}

