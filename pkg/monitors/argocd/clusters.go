package argocd

// Global known clusters - will be set from configuration
var KnownClusters []string

// SetKnownClusters sets the known clusters from configuration.
// This should be called during initialization with the clusters from config.
func SetKnownClusters(clusters []string) {
	KnownClusters = clusters
}
