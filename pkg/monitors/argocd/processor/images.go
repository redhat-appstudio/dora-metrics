// Package processor provides image processing functionality for ArgoCD deployments.
// It handles image validation, change detection, and commit extraction from Docker images.
package processor

import (
	"strings"

	"github.com/redhat-appstudio/dora-metrics/pkg/logger"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/github"
	"github.com/redhat-appstudio/dora-metrics/pkg/storage"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// ImageProcessor handles image-related operations for ArgoCD deployments.
type ImageProcessor struct {
	githubClient github.Client
}

// NewImageProcessor creates a new image processor instance.
func NewImageProcessor(githubClient github.Client) *ImageProcessor {
	return &ImageProcessor{
		githubClient: githubClient,
	}
}

// ExtractValidImages extracts and validates image tags that are commit hashes.
func (ip *ImageProcessor) ExtractValidImages(images []string) []string {
	var validImages []string

	for _, image := range images {
		tag := ip.extractTagFromImage(image)
		if tag != "" {
			if valid, err := ip.githubClient.IsValidCommit(tag); err == nil && valid {
				validImages = append(validImages, image)
			}
		}
	}

	return validImages
}

// FindChangedImages finds images that have changed between deployments.
func (ip *ImageProcessor) FindChangedImages(currentImages, previousImages []string) []string {
	var changedImages []string

	for _, currentImage := range currentImages {
		if !ip.imageExistsInPrevious(currentImage, previousImages) {
			changedImages = append(changedImages, currentImage)
		}
	}

	return changedImages
}

// GetCommitHistoryForImage gets commit history for a specific image.
func (ip *ImageProcessor) GetCommitHistoryForImage(
	app *v1alpha1.Application,
	image string,
	previousImages []string,
) ([]storage.CommitInfo, error) {
	currentTag := ip.extractTagFromImage(image)
	baseImage := ip.getBaseImage(image)

	// Find the previous tag for the same base image
	var previousTag string
	for _, prevImage := range previousImages {
		if ip.getBaseImage(prevImage) == baseImage {
			previousTag = ip.extractTagFromImage(prevImage)
			break
		}
	}

	if previousTag == "" {
		logger.Debugf("No previous tag found for base image %s", baseImage)
		return []storage.CommitInfo{}, nil
	}

	// Find repository URL for the current tag
	repoURL, err := ip.githubClient.FindRepositoryForCommit(currentTag)
	if err != nil {
		logger.Warnf("Failed to find repository for current tag %s: %v", currentTag, err)
		// Try to get from history as fallback
		repoURL = ip.getRepoURLFromHistory(app, currentTag)
		if repoURL == "" {
			return []storage.CommitInfo{}, nil
		}
	}

	// Get commit history between tags
	return ip.githubClient.GetCommitHistoryBetween(previousTag, currentTag, repoURL)
}

// extractTagFromImage extracts the tag from a Docker image reference.
func (ip *ImageProcessor) extractTagFromImage(image string) string {
	parts := strings.Split(image, ":")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

// imageExistsInPrevious checks if an image exists in the previous deployment.
func (ip *ImageProcessor) imageExistsInPrevious(image string, previousImages []string) bool {
	baseImage := ip.getBaseImage(image)
	for _, prevImage := range previousImages {
		if ip.getBaseImage(prevImage) == baseImage {
			return true
		}
	}
	return false
}

// getBaseImage extracts the base image name without tag.
func (ip *ImageProcessor) getBaseImage(image string) string {
	lastColon := strings.LastIndex(image, ":")
	if lastColon == -1 {
		return image
	}
	return image[:lastColon]
}

// getRepoURLFromHistory extracts repository URL from ArgoCD application history.
func (ip *ImageProcessor) getRepoURLFromHistory(app *v1alpha1.Application, commitSHA string) string {
	for _, historyItem := range app.Status.History {
		if historyItem.Revision == commitSHA && historyItem.Source.RepoURL != "" {
			return historyItem.Source.RepoURL
		}
	}
	return ""
}
