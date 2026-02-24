// Package parser provides ArgoCD application parsing functionality.
// It extracts structured information from ArgoCD application objects
// and determines which applications should be monitored based on configuration.
package parser

import (
	"strings"
	"time"

	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/api"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// ApplicationParser implements the ApplicationParser interface.
// It parses ArgoCD applications to extract monitoring-relevant information
// and applies filtering rules based on configuration.
type ApplicationParser struct {
	config *api.Config
}

// NewApplicationParser creates a new application parser instance.
// It takes a configuration object that defines which applications to monitor.
func NewApplicationParser(config *api.Config) api.ApplicationParser {
	return &ApplicationParser{
		config: config,
	}
}

// ParseApplication extracts information from an ArgoCD application.
func (p *ApplicationParser) ParseApplication(app *v1alpha1.Application) (*api.ApplicationInfo, error) {
	// Extract basic information
	appInfo := &api.ApplicationInfo{
		Name:       app.Name,
		Namespace:  app.Namespace,
		Revision:   p.getDeploymentRevision(app),
		DeployedAt: time.Now(),
		Health:     string(app.Status.Health.Status),
		Images:     p.extractImagesFromStatus(&app.Status),
	}

	// Parse component and cluster from application name
	_, appInfo.Component, appInfo.Cluster = p.parseApplicationName(app.Name)

	// Extract environment from source path (e.g., "components/build-service/production/" -> "production")
	appInfo.Environment = p.extractEnvironmentFromSourcePath(app)

	return appInfo, nil
}

// getDeploymentRevision extracts the deployment revision from the application.
// It uses the sync revision, which will be validated against history in the event processor.
func (p *ApplicationParser) getDeploymentRevision(app *v1alpha1.Application) string {
	return app.Status.Sync.Revision
}

// ShouldMonitor determines if an application should be monitored.
func (p *ApplicationParser) ShouldMonitor(app *v1alpha1.Application) bool {
	// Check if monitoring is enabled
	if !p.config.Enabled {
		return false
	}

	// Check if namespace is in the watch list
	if !p.isNamespaceMonitored(app.Namespace) {
		return false
	}

	// Parse application name to get component and cluster
	_, component, cluster := p.parseApplicationName(app.Name)

	// Check if component should be ignored
	if p.isComponentIgnored(component) {
		return false
	}

	// Check if cluster is known
	if !p.isClusterKnown(cluster) {
		return false
	}

	return true
}

// extractImagesFromStatus extracts image information from application status.
func (p *ApplicationParser) extractImagesFromStatus(status *v1alpha1.ApplicationStatus) []string {
	var images []string

	// Extract from status.summary.images
	images = append(images, status.Summary.Images...)

	return images
}

// parseApplicationName parses application name to extract environment, component, and cluster.
// Environment is now extracted from source path, not application name.
func (p *ApplicationParser) parseApplicationName(name string) (string, string, string) {
	// Find cluster suffix
	var cluster string
	for _, knownCluster := range p.config.KnownClusters {
		if strings.HasSuffix(name, "-"+knownCluster) {
			cluster = knownCluster
			break
		}
	}

	if cluster == "" {
		return "", "", ""
	}

	// Remove cluster suffix to get component
	nameWithoutCluster := strings.TrimSuffix(name, "-"+cluster)
	component := nameWithoutCluster

	// Environment is extracted separately from source path
	return "", component, cluster
}

// extractEnvironmentFromSourcePath extracts the environment from the ArgoCD source path.
// Expected path format: "components/<component>/<environment>/" or similar patterns.
// Returns uppercase environment for DevLake compatibility (PRODUCTION, STAGING, DEVELOPMENT).
func (p *ApplicationParser) extractEnvironmentFromSourcePath(app *v1alpha1.Application) string {
	sourcePath := app.Spec.Source.Path
	if sourcePath == "" {
		return "PRODUCTION" // Default fallback
	}

	// Normalize path separators and convert to lowercase for matching
	path := strings.ToLower(sourcePath)

	// Check for known environment patterns in the path
	// Patterns: /production/, /staging/, /development/, /dev/, /stage/, /prod/
	environmentMappings := map[string]string{
		"/production/":  "PRODUCTION",
		"/prod/":        "PRODUCTION",
		"/staging/":     "STAGING",
		"/stage/":       "STAGING",
		"/development/": "DEVELOPMENT",
		"/dev/":         "DEVELOPMENT",
	}

	for pattern, env := range environmentMappings {
		if strings.Contains(path, pattern) {
			return env
		}
	}

	// Also check for environment at the end of path (without trailing slash)
	pathParts := strings.Split(strings.Trim(sourcePath, "/"), "/")
	if len(pathParts) > 0 {
		lastPart := strings.ToLower(pathParts[len(pathParts)-1])
		switch lastPart {
		case "production", "prod":
			return "PRODUCTION"
		case "staging", "stage":
			return "STAGING"
		case "development", "dev":
			return "DEVELOPMENT"
		}
	}

	// Default to PRODUCTION if no environment pattern found
	return "PRODUCTION"
}

// isNamespaceMonitored checks if a namespace should be monitored.
func (p *ApplicationParser) isNamespaceMonitored(namespace string) bool {
	for _, ns := range p.config.Namespaces {
		if ns == namespace {
			return true
		}
	}
	return false
}

// isComponentIgnored checks if a component should be ignored (not monitored).
// Returns true if the component is in the ignore list, false otherwise.
// By default, all components are monitored unless they are in the ignore list.
func (p *ApplicationParser) isComponentIgnored(component string) bool {
	// If component is empty, don't ignore it (let other checks handle it)
	if component == "" {
		return false
	}
	// Check if component is in the ignore list
	for _, ignoredComp := range p.config.ComponentsToIgnore {
		if ignoredComp == component {
			return true
		}
	}
	return false
}

// isClusterKnown checks if a cluster is known.
func (p *ApplicationParser) isClusterKnown(cluster string) bool {
	for _, knownCluster := range p.config.KnownClusters {
		if knownCluster == cluster {
			return true
		}
	}
	return false
}
