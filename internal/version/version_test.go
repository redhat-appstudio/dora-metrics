package version

import (
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetVersion(t *testing.T) {
	// Save original value
	original := BuildVersion
	defer func() { BuildVersion = original }()

	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{
			name:     "default version",
			version:  "v1.0.0",
			expected: "v1.0.0",
		},
		{
			name:     "semantic version with v prefix",
			version:  "v2.5.3",
			expected: "v2.5.3",
		},
		{
			name:     "version without v prefix",
			version:  "1.2.3",
			expected: "1.2.3",
		},
		{
			name:     "empty version",
			version:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			BuildVersion = tt.version
			result := GetVersion()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetShortVersion(t *testing.T) {
	// Save original value
	original := BuildVersion
	defer func() { BuildVersion = original }()

	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{
			name:     "version with v prefix",
			version:  "v1.2.3",
			expected: "1.2.3",
		},
		{
			name:     "version without v prefix",
			version:  "1.2.3",
			expected: "1.2.3",
		},
		{
			name:     "default version",
			version:  "v1.0.0",
			expected: "1.0.0",
		},
		{
			name:     "empty version",
			version:  "",
			expected: "",
		},
		{
			name:     "version with multiple v letters",
			version:  "version-1.0.0",
			expected: "ersion-1.0.0",
		},
		{
			name:     "single v character",
			version:  "v",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			BuildVersion = tt.version
			result := GetShortVersion()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetBuildInfo(t *testing.T) {
	// Save original values
	originalVersion := BuildVersion
	originalTime := BuildTime
	originalCommit := BuildCommit
	defer func() {
		BuildVersion = originalVersion
		BuildTime = originalTime
		BuildCommit = originalCommit
	}()

	tests := []struct {
		name            string
		version         string
		buildTime       string
		commit          string
		expectedContain []string
	}{
		{
			name:      "complete build info",
			version:   "v1.2.3",
			buildTime: "2026-04-20T10:30:00Z",
			commit:    "abc1234",
			expectedContain: []string{
				"v1.2.3",
				"2026-04-20T10:30:00Z",
				"abc1234",
				runtime.Version(),
			},
		},
		{
			name:      "default values",
			version:   "v1.0.0",
			buildTime: "unknown",
			commit:    "unknown",
			expectedContain: []string{
				"v1.0.0",
				"unknown",
				runtime.Version(),
			},
		},
		{
			name:      "partial information",
			version:   "v2.0.0",
			buildTime: "2026-04-01T00:00:00Z",
			commit:    "unknown",
			expectedContain: []string{
				"v2.0.0",
				"2026-04-01T00:00:00Z",
				"unknown",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			BuildVersion = tt.version
			BuildTime = tt.buildTime
			BuildCommit = tt.commit

			result := GetBuildInfo()

			// Verify result is not empty
			assert.NotEmpty(t, result)

			// Verify all expected strings are contained
			for _, expected := range tt.expectedContain {
				assert.True(t, strings.Contains(result, expected),
					"Expected %q to contain %q", result, expected)
			}

			// Verify format (should contain parentheses and commas)
			assert.True(t, strings.Contains(result, "(built:"))
			assert.True(t, strings.Contains(result, "commit:"))
			assert.True(t, strings.Contains(result, "go:"))
		})
	}
}

func TestBuildInfoFormat(t *testing.T) {
	// Test that the format is consistent
	originalVersion := BuildVersion
	originalTime := BuildTime
	originalCommit := BuildCommit
	defer func() {
		BuildVersion = originalVersion
		BuildTime = originalTime
		BuildCommit = originalCommit
	}()

	BuildVersion = "v1.5.0"
	BuildTime = "2026-04-20T12:00:00Z"
	BuildCommit = "def5678"

	result := GetBuildInfo()

	// Verify the expected format structure
	expected := "v1.5.0 (built: 2026-04-20T12:00:00Z, commit: def5678, go: " + runtime.Version() + ")"
	assert.Equal(t, expected, result)
}
