package auth

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewValidator(t *testing.T) {
	validator := NewValidator()

	assert.NotNil(t, validator)
	assert.NotNil(t, validator.httpClient)
	assert.Equal(t, DefaultHTTPTimeout, validator.httpClient.Timeout)
}

func TestExtractEmailFromToken(t *testing.T) {
	validator := NewValidator()

	// Helper to create a mock JWT token
	createJWT := func(claims map[string]interface{}) string {
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))

		payload, _ := json.Marshal(claims)
		encodedPayload := base64.RawURLEncoding.EncodeToString(payload)

		signature := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))

		return header + "." + encodedPayload + "." + signature
	}

	tests := []struct {
		name        string
		token       string
		expectedErr bool
		expected    string
	}{
		{
			name: "valid token with email claim",
			token: createJWT(map[string]interface{}{
				"email": "user@redhat.com",
				"sub":   "user-id-123",
			}),
			expectedErr: false,
			expected:    "user@redhat.com",
		},
		{
			name: "valid token with preferred_username",
			token: createJWT(map[string]interface{}{
				"preferred_username": "alternate@redhat.com",
				"sub":                "user-id-456",
			}),
			expectedErr: false,
			expected:    "alternate@redhat.com",
		},
		{
			name: "email takes priority over preferred_username",
			token: createJWT(map[string]interface{}{
				"email":              "primary@redhat.com",
				"preferred_username": "alternate@redhat.com",
			}),
			expectedErr: false,
			expected:    "primary@redhat.com",
		},
		{
			name:        "invalid token format - not enough parts",
			token:       "invalid.token",
			expectedErr: true,
		},
		{
			name:        "invalid token format - too many parts",
			token:       "part1.part2.part3.part4",
			expectedErr: true,
		},
		{
			name:        "empty token",
			token:       "",
			expectedErr: true,
		},
		{
			name:        "invalid base64 payload",
			token:       "header.!!!invalid-base64!!!.signature",
			expectedErr: true,
		},
		{
			name: "valid JWT but missing email",
			token: createJWT(map[string]interface{}{
				"sub": "user-id-789",
				"iat": 1234567890,
			}),
			expectedErr: true,
		},
		{
			name: "email is not a string",
			token: createJWT(map[string]interface{}{
				"email": 12345,
			}),
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email, err := validator.ExtractEmailFromToken(tt.token)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, email)
			}
		})
	}
}

func TestValidateRedHatEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{
			name:     "valid redhat email",
			email:    "user@redhat.com",
			expected: true,
		},
		{
			name:     "valid redhat email with subdomain",
			email:    "user.name@redhat.com",
			expected: true,
		},
		{
			name:     "non-redhat email",
			email:    "user@gmail.com",
			expected: false,
		},
		{
			name:     "non-redhat email similar domain",
			email:    "user@notredhat.com",
			expected: false,
		},
		{
			name:     "email with redhat in name but different domain",
			email:    "redhat@example.com",
			expected: false,
		},
		{
			name:     "empty email",
			email:    "",
			expected: false,
		},
		{
			name:     "invalid email format",
			email:    "not-an-email",
			expected: false,
		},
		{
			name:     "redhat.com without @",
			email:    "redhat.com",
			expected: false,
		},
		{
			name:     "case sensitivity test",
			email:    "user@REDHAT.COM",
			expected: false, // The function is case-sensitive
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateRedHatEmail(tt.email)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidatorConstants(t *testing.T) {
	// Verify constants are defined correctly
	assert.Equal(t, "https://api.openshift.com/apis/user.openshift.io/v1/users/~", OpenShiftAPIEndpoint)
	assert.NotZero(t, DefaultHTTPTimeout)
}

func TestExtractEmailFromToken_RealWorldScenarios(t *testing.T) {
	validator := NewValidator()

	// Test with various claim structures that might appear in real tokens
	tests := []struct {
		name        string
		claims      map[string]interface{}
		expected    string
		expectError bool
	}{
		{
			name: "full OpenShift token claims",
			claims: map[string]interface{}{
				"iss":                "https://sso.redhat.com",
				"sub":                "f:123456:user",
				"email":              "john.doe@redhat.com",
				"preferred_username": "jdoe",
				"name":               "John Doe",
				"given_name":         "John",
				"family_name":        "Doe",
			},
			expected:    "john.doe@redhat.com",
			expectError: false,
		},
		{
			name: "minimal valid token",
			claims: map[string]interface{}{
				"email": "minimal@redhat.com",
			},
			expected:    "minimal@redhat.com",
			expectError: false,
		},
		{
			name: "token with extra nested objects",
			claims: map[string]interface{}{
				"email": "nested@redhat.com",
				"metadata": map[string]interface{}{
					"createdAt": "2026-01-01",
				},
			},
			expected:    "nested@redhat.com",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a JWT token with the claims
			header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`))
			payload, _ := json.Marshal(tt.claims)
			encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
			signature := base64.RawURLEncoding.EncodeToString([]byte("sig"))
			token := header + "." + encodedPayload + "." + signature

			email, err := validator.ExtractEmailFromToken(token)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, email)
			}
		})
	}
}
