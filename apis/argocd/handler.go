package argocd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	argocd "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/gofiber/fiber/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Handler handles ArgoCD API requests.
type Handler struct {
	// argocdClient is the ArgoCD clientset for accessing applications
	argocdClient *argocdclient.Clientset

	// k8sClient is the Kubernetes client for accessing ArgoCD applications
	k8sClient kubernetes.Interface

	// namespaces are the namespaces to monitor
	namespaces []string

	// componentsToMonitor are components to monitor
	componentsToMonitor []string
}

// NewHandler creates a new ArgoCD API handler.
func NewHandler(argocdClient *argocdclient.Clientset, namespaces, componentsToMonitor []string) (*Handler, error) {
	if argocdClient == nil {
		return nil, errors.New("ArgoCD client is nil")
	}

	// Create Kubernetes client for accessing ArgoCD applications
	k8sClient, err := createK8sClient()
	if err != nil {
		return nil, err
	}

	return &Handler{
		argocdClient:        argocdClient,
		k8sClient:           k8sClient,
		namespaces:          namespaces,
		componentsToMonitor: componentsToMonitor,
	}, nil
}

// createK8sClient creates a Kubernetes client using auto-detection.
func createK8sClient() (kubernetes.Interface, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}

	return kubernetes.NewForConfig(config)
}

// ListApplications handles GET /api/v1/argocd/applications
// Returns applications in Prometheus metrics format for Grafana visualization
func (h *Handler) ListApplications(c *fiber.Ctx) error {
	// Check if ArgoCD client is available
	if h.argocdClient == nil {
		c.Set("Content-Type", "text/plain")
		return c.Status(500).SendString("# ERROR: ArgoCD client not available\n")
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var allApplications []argocd.Application

	// Get applications from each namespace
	for _, namespace := range h.namespaces {
		applications, err := h.getApplicationsFromNamespace(ctx, namespace)
		if err != nil {
			continue
		}
		allApplications = append(allApplications, applications...)
	}

	// Filter out ignored applications
	filteredApplications := h.filterIgnoredApplications(allApplications)

	// Generate Prometheus metrics
	metrics := h.generatePrometheusMetrics(filteredApplications)

	// Set content type for Prometheus
	c.Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	return c.SendString(metrics)
}

// getApplicationsFromNamespace retrieves applications from a specific namespace.
func (h *Handler) getApplicationsFromNamespace(ctx context.Context, namespace string) ([]argocd.Application, error) {
	// List ArgoCD applications in the namespace
	appList, err := h.argocdClient.ArgoprojV1alpha1().Applications(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return appList.Items, nil
}

// filterIgnoredApplications returns all applications without filtering.
// The ArgoCD API should show all applications, filtering is only used for monitoring.
func (h *Handler) filterIgnoredApplications(applications []argocd.Application) []argocd.Application {
	// Return all applications - no filtering for API endpoints
	return applications
}

// determineEnvironment determines the environment based on source path.
func (h *Handler) determineEnvironment(sourcePath string) string {
	if sourcePath == "" {
		return "production"
	}

	path := strings.ToLower(sourcePath)
	if strings.Contains(path, "staging") || strings.Contains(path, "stage") {
		return "staging"
	} else if strings.Contains(path, "dev") || strings.Contains(path, "development") {
		return "development"
	}

	return "production"
}

// generatePrometheusMetrics generates Prometheus metrics format for ArgoCD applications
func (h *Handler) generatePrometheusMetrics(applications []argocd.Application) string {
	var metrics strings.Builder

	// Add proper Prometheus metric headers
	metrics.WriteString("# HELP argocd_application_info Comprehensive information about ArgoCD applications\n")
	metrics.WriteString("# TYPE argocd_application_info gauge\n")
	metrics.WriteString("# HELP argocd_application_count_total Total number of applications per cluster and environment\n")
	metrics.WriteString("# TYPE argocd_application_count_total gauge\n")
	metrics.WriteString("\n")

	// Count applications by cluster and environment
	clusterEnvCount := make(map[string]map[string]int)

	// Generate metrics for each application
	for _, app := range applications {
		// Skip applications with empty names
		if app.Name == "" {
			continue
		}

		// Determine environment based on source path
		var sourcePath string
		if app.Spec.Source != nil {
			sourcePath = app.Spec.Source.Path
		}
		environment := h.determineEnvironment(sourcePath)

		// Parse application name to extract cluster and component
		// For API purposes, we'll use a simple parsing approach
		parts := strings.Split(app.Name, "-")
		var clusterName, componentName string
		if len(parts) >= 2 {
			componentName = parts[len(parts)-2]
			clusterName = parts[len(parts)-1]
		} else {
			componentName = app.Name
			clusterName = "unknown"
		}

		if clusterName == "" {
			clusterName = "unknown"
		}
		if componentName == "" {
			componentName = "unknown"
		}

		// Count for cluster/environment metrics
		if clusterEnvCount[clusterName] == nil {
			clusterEnvCount[clusterName] = make(map[string]int)
		}
		clusterEnvCount[clusterName][environment]++

		// Determine health and sync status values
		healthValue := 0
		if app.Status.Health.Status == "Healthy" {
			healthValue = 1
		}

		syncValue := 0
		if app.Status.Sync.Status == "Synced" {
			syncValue = 1
		}

		// Get image information
		imageInfo := "N/A"
		if len(app.Status.Summary.Images) > 0 {
			imageInfo = strings.Join(app.Status.Summary.Images, ",")
		}

		// Single comprehensive application info metric with all information
		metrics.WriteString(fmt.Sprintf("argocd_application_info{namespace=\"%s\",name=\"%s\",cluster=\"%s\",environment=\"%s\",component=\"%s\",health_status=\"%s\",health_value=\"%d\",sync_status=\"%s\",sync_value=\"%d\",image=\"%s\"} 1\n",
			app.Namespace, app.Name, clusterName, environment, componentName,
			app.Status.Health.Status, healthValue,
			app.Status.Sync.Status, syncValue,
			imageInfo))
	}

	// Add cluster/environment count metrics
	metrics.WriteString("\n")
	for clusterName, envCounts := range clusterEnvCount {
		for environment, count := range envCounts {
			metrics.WriteString(fmt.Sprintf("argocd_application_count_total{cluster=\"%s\",environment=\"%s\"} %d\n",
				clusterName, environment, count))
		}
	}

	// Add total count metric
	totalCount := len(applications)
	metrics.WriteString(fmt.Sprintf("argocd_application_count_total{cluster=\"all\",environment=\"all\"} %d\n", totalCount))

	return metrics.String()
}
