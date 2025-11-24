// Package api provides the fundamental types and interfaces for ArgoCD monitoring.
// It defines the API abstractions that other modules implement, ensuring a clean
// separation of concerns and making the system easy to extend and test.
package api

import (
	"context"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argocd "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/watch"
)

// Config holds configuration for ArgoCD application monitoring.
// It specifies which namespaces to watch, which components to monitor,
// and other monitoring settings.
type Config struct {
	// Enabled controls whether ArgoCD monitoring is active
	Enabled bool `json:"enabled"`

	// Namespaces lists Kubernetes namespaces to watch for ArgoCD applications
	Namespaces []string `json:"namespaces"`

	// ComponentsToIgnore lists component names to exclude from monitoring
	// All other components will be monitored across all clusters
	ComponentsToIgnore []string `json:"components_to_ignore"`

	// KnownClusters lists known cluster names for parsing application names
	KnownClusters []string `json:"known_clusters"`

	// RepositoryBlacklist lists repository URLs to exclude from commit processing
	// Commits from these repositories will be filtered out from deployment payloads
	RepositoryBlacklist []string `json:"repository_blacklist"`
}

// ApplicationInfo contains parsed information from an ArgoCD application.
// It represents the essential data extracted from an application for monitoring purposes.
type ApplicationInfo struct {
	// Name is the ArgoCD application name
	Name string

	// Namespace is the Kubernetes namespace where the application is deployed
	Namespace string

	// Environment is the detected environment (e.g., "production", "staging")
	Environment string

	// Component is the extracted component name (e.g., "build-service")
	Component string

	// Cluster is the detected cluster name (e.g., "kflux-prd-rh03")
	Cluster string

	// Revision is the Git revision/commit hash that was deployed
	Revision string

	// DeployedAt is when the deployment occurred
	DeployedAt time.Time

	// Health is the application health status
	Health string

	// Images are the container images used in the deployment
	Images []string
}

// EventHandler defines the interface for handling ArgoCD application events.
// Implementations should process different types of events (ADDED, MODIFIED, DELETED)
// and perform appropriate actions based on the event type and application state.
type EventHandler interface {
	// HandleEvent processes a single ArgoCD application event
	HandleEvent(ctx context.Context, event watch.Event, app *v1alpha1.Application) error
}

// ApplicationParser defines the interface for parsing ArgoCD applications.
// It extracts relevant information from application objects and determines
// whether an application should be monitored based on configuration rules.
type ApplicationParser interface {
	// ParseApplication extracts structured information from an ArgoCD application
	ParseApplication(app *v1alpha1.Application) (*ApplicationInfo, error)

	// ShouldMonitor determines if an application should be monitored based on configuration
	ShouldMonitor(app *v1alpha1.Application) bool
}

// Client defines the interface for ArgoCD API operations.
// It provides access to the ArgoCD clientset and configuration information
// needed for monitoring operations.
type Client interface {
	// GetArgoCDClient returns the ArgoCD Kubernetes clientset
	GetArgoCDClient() *argocd.Clientset

	// GetNamespaces returns the list of namespaces to monitor
	GetNamespaces() []string
}

// Monitor defines the interface for ArgoCD application monitoring.
// It provides lifecycle management for the monitoring service, allowing
// it to be started and stopped cleanly.
type Monitor interface {
	// Start begins the monitoring process
	Start(ctx context.Context) error

	// Stop gracefully shuts down the monitoring process
	Stop()
}

// NewApplicationParser creates a new application parser instance.
// This is a factory function that returns the default implementation
// of the ApplicationParser interface.
func NewApplicationParser(config *Config) ApplicationParser {
	// This would be implemented in the parser package
	return nil
}
