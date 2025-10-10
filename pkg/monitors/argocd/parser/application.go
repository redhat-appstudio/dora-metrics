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

	// Parse environment, component, and cluster from application name
	appInfo.Environment, appInfo.Component, appInfo.Cluster = p.parseApplicationName(app.Name)

	return appInfo, nil
}

// getDeploymentRevision extracts the actual deployment revision from the application.
// It uses the sync revision as the current deployment revision.
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

	// Check if component should be monitored
	if !p.isComponentMonitored(component) {
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

	// For now, set environment to "production" since we don't have environment info in the name
	environment := "production"

	return environment, component, cluster
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

// isComponentMonitored checks if a component should be monitored.
func (p *ApplicationParser) isComponentMonitored(component string) bool {
	for _, comp := range p.config.ComponentsToMonitor {
		if comp == component {
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
