package argocd

import (
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/api"
	"github.com/redhat-appstudio/dora-metrics/pkg/storage"
)

// NewMonitor creates a new ArgoCD monitor using the factory pattern.
// This is the main entry point for creating ArgoCD monitoring instances.
// It takes configuration and storage client as parameters and returns
// a fully configured monitor ready to start.
func NewMonitor(config *api.Config, storage *storage.RedisClient) (api.Monitor, error) {
	factory := NewFactory(config, storage)
	return factory.CreateMonitor()
}
