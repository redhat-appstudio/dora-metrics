package argocd

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	argocd "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/gofiber/fiber/v2"
	"github.com/redhat-appstudio/dora-metrics/pkg/auth"
	"github.com/redhat-appstudio/dora-metrics/pkg/logger"
	"github.com/toon-format/toon-go"
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

	// componentsToIgnore are components to exclude from monitoring
	componentsToIgnore []string

	// knownClusters are the known cluster names for proper extraction
	knownClusters []string

	// offlineToken is the OpenShift offline token for authentication
	offlineToken string

	// authValidator validates tokens and extracts email
	authValidator *auth.Validator
}

// NewHandler creates a new ArgoCD API handler.
func NewHandler(argocdClient *argocdclient.Clientset, namespaces, componentsToIgnore, knownClusters []string, offlineToken string) (*Handler, error) {
	if argocdClient == nil {
		return nil, errors.New("ArgoCD client is nil")
	}

	// Create Kubernetes client for accessing ArgoCD applications
	k8sClient, err := createK8sClient()
	if err != nil {
		return nil, err
	}

	return &Handler{
		argocdClient:       argocdClient,
		k8sClient:          k8sClient,
		namespaces:         namespaces,
		componentsToIgnore: componentsToIgnore,
		knownClusters:      knownClusters,
		offlineToken:       offlineToken,
		authValidator:      auth.NewValidator(),
	}, nil
}

// extractClusterName extracts the cluster name from an application name by matching against known clusters.
func (h *Handler) extractClusterName(appName string) string {
	// Try to find a known cluster name in the application name
	for _, cluster := range h.knownClusters {
		if strings.Contains(appName, cluster) {
			return cluster
		}
	}

	// Fallback to the old logic if no known cluster is found
	parts := strings.Split(appName, "-")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}

	return "unknown"
}

// extractComponentName extracts the component name from an application name.
// Since we now monitor all components by default, we extract the component
// by removing the cluster suffix from the application name.
func (h *Handler) extractComponentName(appName string) string {
	// Try to find a known cluster name in the application name and remove it
	for _, cluster := range h.knownClusters {
		if strings.HasSuffix(appName, "-"+cluster) {
			// Remove the cluster suffix to get the component name
			return strings.TrimSuffix(appName, "-"+cluster)
		}
	}

	// Fallback to the old logic if no known cluster is found
	parts := strings.Split(appName, "-")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}

	return appName
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

// ApplicationInfo represents application information in JSON format
type ApplicationInfo struct {
	Namespace    string   `json:"namespace" toon:"namespace"`
	Name         string   `json:"name" toon:"name"`
	Component    string   `json:"component" toon:"component"`
	HealthStatus string   `json:"health_status" toon:"health_status"`
	HealthValue  int      `json:"health_value" toon:"health_value"`
	SyncStatus   string   `json:"sync_status" toon:"sync_status"`
	SyncValue    int      `json:"sync_value" toon:"sync_value"`
	Images       []string `json:"images" toon:"images"`
}

// ClusterApplications represents applications grouped by cluster
type ClusterApplications struct {
	Cluster      string            `json:"cluster" toon:"cluster"`
	Applications []ApplicationInfo `json:"applications" toon:"applications"`
	Count        int               `json:"count" toon:"count"`
}

// ApplicationsResponse represents the JSON response structure grouped by cluster
type ApplicationsResponse struct {
	Clusters   []ClusterApplications `json:"clusters" toon:"clusters"`
	TotalCount int                   `json:"total_count" toon:"total_count"`
}

// validateAuth validates the Authorization header and checks for @redhat.com email
// Returns true if authentication is successful, false otherwise
// If false, the error response is already sent to the client
func (h *Handler) validateAuth(c *fiber.Ctx) bool {
	// Get Authorization header
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		c.Status(401).JSON(fiber.Map{
			"error": "Authorization header is required",
		})
		return false
	}

	// Extract Bearer token
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.Status(401).JSON(fiber.Map{
			"error": "Invalid Authorization header format. Expected: Bearer <token>",
		})
		return false
	}

	token := parts[1]

	// Validate token against OpenShift API and extract email using common auth library
	email, err := h.authValidator.ValidateTokenAndExtractEmail(token)
	if err != nil {
		logger.Warnf("Token validation failed: %v", err)
		c.Status(401).JSON(fiber.Map{
			"error": "Invalid or expired token",
		})
		return false
	}

	// Validate email domain - must be @redhat.com
	if !auth.ValidateRedHatEmail(email) {
		c.Status(403).JSON(fiber.Map{
			"error": "Access denied. Only @redhat.com email addresses are allowed",
		})
		return false
	}

	logger.Debugf("Authenticated request from: %s", email)
	return true
}

// ListApplicationsJSON handles GET /api/v1/argocd/applications
// Returns applications in JSON format
func (h *Handler) ListApplicationsJSON(c *fiber.Ctx) error {
	// Validate authentication
	if !h.validateAuth(c) {
		return nil // Error response already sent
	}

	// Check if ArgoCD client is available
	if h.argocdClient == nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "ArgoCD client not available",
		})
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

	// Convert to JSON format
	response := h.generateJSONResponse(filteredApplications)

	c.Set("Content-Type", "application/json")
	return c.JSON(response)
}

// ListApplicationsTOON handles GET /api/v1/argocd/applications.toon
// Returns applications in TOON format
func (h *Handler) ListApplicationsTOON(c *fiber.Ctx) error {
	// Validate authentication
	if !h.validateAuth(c) {
		return nil // Error response already sent
	}

	// Check if ArgoCD client is available
	if h.argocdClient == nil {
		return c.Status(500).SendString("error: ArgoCD client not available\n")
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

	// Convert to response format (same structure as JSON)
	response := h.generateJSONResponse(filteredApplications)

	// Convert to TOON format using official library
	toonData, err := toon.Marshal(response, toon.WithLengthMarkers(true))
	if err != nil {
		return c.Status(500).SendString("error: failed to marshal TOON\n")
	}

	c.Set("Content-Type", "text/plain")
	return c.Send(toonData)
}

// generateJSONResponse generates JSON response from ArgoCD applications grouped by cluster
func (h *Handler) generateJSONResponse(applications []argocd.Application) ApplicationsResponse {
	// Group applications by cluster
	clusterMap := make(map[string][]ApplicationInfo)

	// Process each application
	for _, app := range applications {
		// Skip applications with empty names
		if app.Name == "" {
			continue
		}

		// Extract cluster and component names
		clusterName := h.extractClusterName(app.Name)
		componentName := h.extractComponentName(app.Name)

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
		images := app.Status.Summary.Images
		if len(images) == 0 {
			images = []string{}
		}

		// Create application info (without cluster field since it's in the parent)
		appInfo := ApplicationInfo{
			Namespace:    app.Namespace,
			Name:         app.Name,
			Component:    componentName,
			HealthStatus: string(app.Status.Health.Status),
			HealthValue:  healthValue,
			SyncStatus:   string(app.Status.Sync.Status),
			SyncValue:    syncValue,
			Images:       images,
		}

		// Add to cluster map
		clusterMap[clusterName] = append(clusterMap[clusterName], appInfo)
	}

	// Build clusters array
	var clusters []ClusterApplications
	for clusterName, apps := range clusterMap {
		clusters = append(clusters, ClusterApplications{
			Cluster:      clusterName,
			Applications: apps,
			Count:        len(apps),
		})
	}

	return ApplicationsResponse{
		Clusters:   clusters,
		TotalCount: len(applications),
	}
}
