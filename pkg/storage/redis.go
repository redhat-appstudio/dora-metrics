package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redhat-appstudio/dora-metrics/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// RedisClient handles all Redis operations for deployment history storage.
type RedisClient struct {
	client    *redis.Client
	keyPrefix string
}

// NewRedisClient creates a new Redis client with the provided configuration.
// It initializes the connection and validates connectivity.
func NewRedisClient(config RedisConfig) (*RedisClient, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("Redis storage is disabled")
	}

	if config.Address == "" {
		return nil, fmt.Errorf("Redis address is required")
	}

	// Create Redis client with optimized settings
	rdb := redis.NewClient(&redis.Options{
		Addr:         config.Address,
		Password:     config.Password,
		DB:           config.Database,
		PoolSize:     10, // Connection pool size
		MinIdleConns: 2,  // Minimum idle connections
		MaxRetries:   3,  // Maximum retries for failed commands
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Infof("Redis storage client connected successfully to %s", config.Address)

	return &RedisClient{
		client:    rdb,
		keyPrefix: config.KeyPrefix,
	}, nil
}

// Close closes the Redis connection.
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// buildKey efficiently builds Redis keys using strings.Builder
func (r *RedisClient) buildKey(parts ...string) string {
	var builder strings.Builder
	builder.WriteString(r.keyPrefix)
	for _, part := range parts {
		builder.WriteByte(':')
		builder.WriteString(part)
	}
	return builder.String()
}

// StoreDeployment stores a deployment record in Redis.
func (r *RedisClient) StoreDeployment(ctx context.Context, deployment *DeploymentRecord) error {
	key := r.buildKey(deployment.ApplicationName, deployment.ClusterName)

	// Convert to JSON
	data, err := json.Marshal(deployment)
	if err != nil {
		return fmt.Errorf("failed to marshal deployment record: %w", err)
	}

	// Store with expiration (30 days)
	expiration := 30 * 24 * time.Hour
	if err := r.client.Set(ctx, key, data, expiration).Err(); err != nil {
		return fmt.Errorf("failed to store deployment record: %w", err)
	}

	logger.Debugf("Stored deployment record for %s/%s (revision: %s)", deployment.ApplicationName, deployment.ClusterName, deployment.Revision)
	return nil
}

// GetDeployment retrieves a deployment record from Redis.
func (r *RedisClient) GetDeployment(ctx context.Context, appName, clusterName string) (*DeploymentRecord, error) {
	key := r.buildKey(appName, clusterName)

	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("deployment not found")
		}
		return nil, fmt.Errorf("failed to get deployment record: %w", err)
	}

	var deployment DeploymentRecord
	if err := json.Unmarshal([]byte(data), &deployment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deployment record: %w", err)
	}

	return &deployment, nil
}

// IsNewDeployment checks if a deployment is new (not previously stored).
func (r *RedisClient) IsNewDeployment(ctx context.Context, appName, clusterName, revision string) (bool, error) {
	existing, err := r.GetDeployment(ctx, appName, clusterName)
	if err != nil {
		return false, err
	}

	if existing == nil {
		return true, nil // No previous deployment
	}

	return existing.Revision != revision, nil
}

// GetPreviousDeployment gets the most recent previous deployment for an application
func (r *RedisClient) GetPreviousDeployment(ctx context.Context, appName, clusterName string) (*DeploymentRecord, error) {
	key := fmt.Sprintf("%s:deployment:%s:%s", r.keyPrefix, appName, clusterName)

	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // No previous deployment
		}
		return nil, err
	}

	var deployment DeploymentRecord
	if err := json.Unmarshal([]byte(val), &deployment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deployment: %w", err)
	}

	return &deployment, nil
}

// SetCache stores a value in Redis cache with TTL
func (r *RedisClient) SetCache(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	cacheKey := r.buildKey("cache", key)

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal cache value: %w", err)
	}

	return r.client.Set(ctx, cacheKey, data, ttl).Err()
}

// GetCache retrieves a value from Redis cache
func (r *RedisClient) GetCache(ctx context.Context, key string, dest interface{}) (bool, error) {
	cacheKey := r.buildKey("cache", key)

	data, err := r.client.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil // Not found
		}
		return false, fmt.Errorf("failed to get cache value: %w", err)
	}

	if err := json.Unmarshal([]byte(data), dest); err != nil {
		return false, fmt.Errorf("failed to unmarshal cache value: %w", err)
	}

	return true, nil
}

// MarkCommitAsProcessed marks a commit as processed for a specific application and cluster in Redis
func (r *RedisClient) MarkCommitAsProcessed(ctx context.Context, commitSHA string, appName, clusterName string) error {
	key := r.buildKey("processed", commitSHA, appName, clusterName)

	// Store with a long expiration (30 days) to track processed commits per application+cluster
	expiration := 30 * 24 * time.Hour
	return r.client.Set(ctx, key, "processed", expiration).Err()
}

// IsCommitProcessed checks if a commit has been processed for a specific application and cluster
func (r *RedisClient) IsCommitProcessed(ctx context.Context, commitSHA string, appName, clusterName string) (bool, error) {
	key := r.buildKey("processed", commitSHA, appName, clusterName)

	_, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil // Not found, not processed
		}
		return false, fmt.Errorf("failed to check if commit is processed: %w", err)
	}

	return true, nil
}

// MarkDevLakeCommitAsProcessed marks a commit as sent to DevLake for a specific component
func (r *RedisClient) MarkDevLakeCommitAsProcessed(ctx context.Context, commitSHA string, component string) error {
	key := r.buildKey("devlake", commitSHA, component)

	// Store with a long expiration (30 days) to track DevLake processed commits per component
	expiration := 30 * 24 * time.Hour
	return r.client.Set(ctx, key, "processed", expiration).Err()
}

// IsDevLakeCommitProcessed checks if a commit has been sent to DevLake for a specific component
func (r *RedisClient) IsDevLakeCommitProcessed(ctx context.Context, commitSHA string, component string) (bool, error) {
	key := r.buildKey("devlake", commitSHA, component)

	_, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil // Not found, not processed
		}
		return false, fmt.Errorf("failed to check if commit is processed in DevLake: %w", err)
	}

	return true, nil
}
