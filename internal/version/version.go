package version

import (
	"fmt"
	"runtime"
)

// Version information that can be set at build time
var (
	// These can be set via ldflags during build:
	// go build -ldflags "-X main.BuildVersion=v1.2.3 -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.BuildCommit=$(git rev-parse --short HEAD)"
	BuildVersion = "v1.0.0"
	BuildTime    = "unknown"
	BuildCommit  = "unknown"
)

// GetVersion returns the current version string.
// This is typically set at build time using ldflags.
func GetVersion() string {
	return BuildVersion
}

// GetBuildInfo returns comprehensive build information including version, time, and commit.
// This provides detailed build metadata for debugging and identification purposes.
func GetBuildInfo() string {
	return fmt.Sprintf("%s (built: %s, commit: %s, go: %s)",
		BuildVersion, BuildTime, BuildCommit, runtime.Version())
}

// GetShortVersion returns just the version number without the "v" prefix.
// This is useful for cases where only the numeric version is needed.
func GetShortVersion() string {
	if len(BuildVersion) > 0 && BuildVersion[0] == 'v' {
		return BuildVersion[1:]
	}
	return BuildVersion
}
