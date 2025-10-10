package storage

import "fmt"

// NewManager creates a new Redis client with the provided configuration.
// This is a convenience function that creates a RedisClient directly.
func NewManager(config StorageConfig) (*RedisClient, error) {
	if !config.Redis.Enabled {
		return nil, fmt.Errorf("Redis storage is required but not enabled")
	}

	return NewRedisClient(config.Redis)
}
