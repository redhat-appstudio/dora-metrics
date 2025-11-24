package storage

import "time"

// CommitInfo represents a commit with its SHA, message, repository URL, and creation date
type CommitInfo struct {
	SHA       string    `json:"sha"`
	Message   string    `json:"message"`
	RepoURL   string    `json:"repo_url,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// DeploymentRecord represents a single deployment record stored in Redis.
// It contains all the information about a deployment that needs to be tracked.
type DeploymentRecord struct {
	// ApplicationName is the full ArgoCD application name
	ApplicationName string `json:"application_name"`

	// Namespace is the Kubernetes namespace where the application is deployed
	Namespace string `json:"namespace"`

	// ComponentName is the extracted component name (e.g., "pulp-access-controller")
	ComponentName string `json:"component_name"`

	// ClusterName is the extracted cluster name (e.g., "pentest-p01")
	ClusterName string `json:"cluster_name"`

	// Revision is the Git revision/commit hash that was deployed
	Revision string `json:"revision"`

	// Images are the container images used in the deployment
	Images []string `json:"images"`

	// CommitHistory contains the commit history between this and previous deployment
	// Note: This field is used for logging only, not stored in database
	CommitHistory []string `json:"commit_history,omitempty"`

	// DeployedAt is when the deployment occurred
	DeployedAt time.Time `json:"deployed_at"`

	// Environment is the detected environment (production/staging)
	Environment string `json:"environment"`

	// Health is the application health status at deployment time
	Health string `json:"health"`

	// Timestamp is when this record was created in Redis
	Timestamp time.Time `json:"timestamp"`
}

// StorageConfig holds configuration for the storage backend.
type StorageConfig struct {
	// Redis configuration
	Redis RedisConfig `json:"redis"`
}

// RedisConfig holds Redis-specific configuration.
type RedisConfig struct {
	// Enabled indicates if Redis storage is enabled
	Enabled bool `json:"enabled"`

	// Address is the Redis server address (host:port)
	Address string `json:"address"`

	// Password is the Redis password (optional)
	Password string `json:"password"`

	// Database is the Redis database number (0-15)
	Database int `json:"database"`

	// KeyPrefix is the prefix for all Redis keys
	KeyPrefix string `json:"key_prefix"`
}
