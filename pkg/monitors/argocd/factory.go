package argocd

import (
	"fmt"
	"os"
	"path/filepath"

	argocd "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/api"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/github"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/parser"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/processor"
	"github.com/redhat-appstudio/dora-metrics/pkg/storage"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Factory creates ArgoCD monitoring components.
// It provides a centralized way to create and configure all the components
// needed for ArgoCD monitoring, ensuring proper dependency injection.
type Factory struct {
	config  *api.Config
	storage *storage.RedisClient
}

// NewFactory creates a new factory instance.
// It takes configuration and storage client as parameters.
func NewFactory(config *api.Config, storage *storage.RedisClient) *Factory {
	return &Factory{
		config:  config,
		storage: storage,
	}
}

// CreateMonitor creates a new ArgoCD monitor instance.
func (f *Factory) CreateMonitor() (api.Monitor, error) {
	client, err := f.createClient()
	if err != nil {
		return nil, err
	}

	githubClient := f.createGitHubClient()
	eventHandler := f.createEventHandler(githubClient, client)
	parser := f.createParser()
	watcher := f.createWatcher(client, eventHandler, parser)

	return watcher, nil
}

// argocdClient implements the api.Client interface.
type argocdClient struct {
	argocdClient *argocd.Clientset
	namespaces   []string
}

func (c *argocdClient) GetArgoCDClient() *argocd.Clientset {
	return c.argocdClient
}

func (c *argocdClient) GetNamespaces() []string {
	return c.namespaces
}

// createClient creates an ArgoCD client.
func (f *Factory) createClient() (api.Client, error) {
	// Get Kubernetes REST config
	restConfig, err := getK8sRestConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes config: %w", err)
	}

	// Create ArgoCD clientset
	argocdClientset, err := argocd.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ArgoCD clientset: %w", err)
	}

	return &argocdClient{
		argocdClient: argocdClientset,
		namespaces:   f.config.Namespaces,
	}, nil
}

// CreateArgoCDClient creates a simple ArgoCD client for API endpoints.
// This is a convenience function for creating clients outside the factory pattern.
func CreateArgoCDClient(config *api.Config) (api.Client, error) {
	// Get Kubernetes REST config
	restConfig, err := getK8sRestConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes config: %w", err)
	}

	// Create ArgoCD clientset
	argocdClientset, err := argocd.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ArgoCD clientset: %w", err)
	}

	return &argocdClient{
		argocdClient: argocdClientset,
		namespaces:   config.Namespaces,
	}, nil
}

// getK8sRestConfig gets Kubernetes REST config using auto-detection.
func getK8sRestConfig() (*rest.Config, error) {
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

	return config, nil
}

// createGitHubClient creates a GitHub client.
func (f *Factory) createGitHubClient() github.Client {
	// Get GitHub token from environment variable
	githubToken := os.Getenv("GITHUB_TOKEN")

	// Create GitHub client with configuration
	config := &github.Config{
		Token: githubToken,
	}

	return github.NewClient(config)
}

// createEventHandler creates an event handler.
func (f *Factory) createEventHandler(githubClient github.Client, argocdClient api.Client) api.EventHandler {
	return processor.NewEventProcessor(f.config, f.storage, githubClient, argocdClient)
}

// createParser creates an application parser.
func (f *Factory) createParser() api.ApplicationParser {
	return parser.NewApplicationParser(f.config)
}

// createWatcher creates a watcher instance.
func (f *Factory) createWatcher(client api.Client, eventHandler api.EventHandler, parser api.ApplicationParser) api.Monitor {
	return api.NewArgoCDWatcher(client, eventHandler, parser, 100) // Increased workers to process events faster
}
