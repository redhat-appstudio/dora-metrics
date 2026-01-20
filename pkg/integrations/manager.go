package integrations

import (
	"context"
	"fmt"
	"sync"

	"github.com/redhat-appstudio/dora-metrics/internal/config"
	"github.com/redhat-appstudio/dora-metrics/pkg/logger"
)

// Manager handles all integrations in the system
type Manager struct {
	integrations map[string]Integration
	mu           sync.RWMutex
}

// Integration represents a generic integration interface
type Integration interface {
	IsEnabled() bool
	SendIncidentEvent(ctx context.Context, incident IncidentData, count int) error
	CloseIncident(ctx context.Context, incidentID string) error
	SendDeploymentEvent(ctx context.Context, deployment DevLakeCICDDeployment) error
}

var (
	globalManager *Manager
	once          sync.Once
)

// GetManager returns the global integration manager instance
func GetManager() *Manager {
	once.Do(func() {
		globalManager = &Manager{
			integrations: make(map[string]Integration),
		}
	})
	return globalManager
}

// RegisterIntegration registers a new integration
func (m *Manager) RegisterIntegration(name string, integration Integration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.integrations[name] = integration
}

// GetIntegration returns an integration by name
func (m *Manager) GetIntegration(name string) (Integration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	integration, exists := m.integrations[name]
	if !exists {
		return nil, fmt.Errorf("integration %s not found", name)
	}
	return integration, nil
}

// SendIncidentEventToDevLake sends an incident event to DevLake
func (m *Manager) SendIncidentEventToDevLake(ctx context.Context, incident IncidentData, count int) error {
	integration, err := m.GetIntegration("devlake")
	if err != nil {
		return fmt.Errorf("failed to get devlake integration: %w", err)
	}

	if !integration.IsEnabled() {
		return fmt.Errorf("devlake integration is disabled")
	}

	return integration.SendIncidentEvent(ctx, incident, count)
}

// CloseIncidentInDevLake closes an incident in DevLake
func (m *Manager) CloseIncidentInDevLake(ctx context.Context, incidentID string) error {
	integration, err := m.GetIntegration("devlake")
	if err != nil {
		return fmt.Errorf("failed to get devlake integration: %w", err)
	}

	if !integration.IsEnabled() {
		return fmt.Errorf("devlake integration is disabled")
	}

	return integration.CloseIncident(ctx, incidentID)
}

// SendDeploymentEventToDevLake sends a deployment event to DevLake
func (m *Manager) SendDeploymentEventToDevLake(ctx context.Context, deployment DevLakeCICDDeployment) error {
	integration, err := m.GetIntegration("devlake")
	if err != nil {
		return fmt.Errorf("failed to get devlake integration: %w", err)
	}

	if !integration.IsEnabled() {
		return fmt.Errorf("devlake integration is disabled")
	}

	return integration.SendDeploymentEvent(ctx, deployment)
}

// RegisterDevLakeIntegration registers a DevLake integration
func (m *Manager) RegisterDevLakeIntegration(baseURL string, projectID string, enabled bool, timeoutSeconds int, teams []config.TeamConfig) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30 // Default timeout
	}
	devlakeIntegration := NewDevLakeIntegration(baseURL, projectID, enabled, timeoutSeconds, teams)
	m.RegisterIntegration("devlake", devlakeIntegration)
	
	// Log team configuration summary
	if enabled && len(teams) > 0 {
		totalComponents := 0
		for _, team := range teams {
			totalComponents += len(team.ArgocdComponents)
		}
		logger.Infof("DevLake integration registered with %d team(s) managing %d total component(s)", len(teams), totalComponents)
	}
}
