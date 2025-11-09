package integrations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/redhat-appstudio/dora-metrics/internal/config"
	"github.com/redhat-appstudio/dora-metrics/pkg/logger"
)

// DevLakeIssue represents the DevLake issue payload structure
// Following the official DevLake webhook API documentation
type DevLakeIssue struct {
	// Issue's URL (optional)
	URL string `json:"url,omitempty"`

	// Issue's key, needs to be unique in a connection (required)
	IssueKey string `json:"issueKey"`

	// Issue title (required)
	Title string `json:"title"`

	// Issue description (optional)
	Description string `json:"description,omitempty"`

	// Issue's epic (optional)
	EpicKey string `json:"epicKey,omitempty"`

	// Type, such as INCIDENT, BUG, REQUIREMENT (optional)
	Type string `json:"type,omitempty"`

	// Issue's status. Must be one of TODO DONE IN_PROGRESS (required)
	Status string `json:"status"`

	// Status in your tool, such as created/open/closed/... (required)
	OriginalStatus string `json:"originalStatus"`

	// Story point (optional)
	StoryPoint string `json:"storyPoint,omitempty"`

	// Resolved date, Format should be 2020-01-01T12:00:00+00:00 (optional)
	ResolutionDate string `json:"resolutionDate,omitempty"`

	// Created date, Format should be 2020-01-01T12:00:00+00:00 (required)
	CreatedDate string `json:"createdDate"`

	// Last updated date, Format should be 2020-01-01T12:00:00+00:00 (optional)
	UpdatedDate string `json:"updatedDate,omitempty"`

	// How long from this issue accepted to develop (optional)
	LeadTimeMinutes int `json:"leadTimeMinutes,omitempty"`

	// Parent issue key (optional)
	ParentIssueKey string `json:"parentIssueKey,omitempty"`

	// Priority (optional)
	Priority string `json:"priority,omitempty"`

	// Original estimate minutes (optional)
	OriginalEstimateMinutes int `json:"originalEstimateMinutes,omitempty"`

	// Time spent minutes (optional)
	TimeSpentMinutes int `json:"timeSpentMinutes,omitempty"`

	// Time remaining minutes (optional)
	TimeRemainingMinutes int `json:"timeRemainingMinutes,omitempty"`

	// The user id of the creator (optional)
	CreatorID string `json:"creatorId,omitempty"`

	// The username of the creator, it will just be used to display (optional)
	CreatorName string `json:"creatorName,omitempty"`

	// Assignee ID (optional)
	AssigneeID string `json:"assigneeId,omitempty"`

	// Assignee name (optional)
	AssigneeName string `json:"assigneeName,omitempty"`

	// Severity (optional)
	Severity string `json:"severity,omitempty"`

	// Component (optional)
	Component string `json:"component,omitempty"`
}

// DevLakeDeploymentCommit represents a deployment commit in DevLake format
type DevLakeDeploymentCommit struct {
	RepoURL      string  `json:"repoUrl"`
	RefName      string  `json:"refName"`
	StartedDate  string  `json:"startedDate"`
	FinishedDate string  `json:"finishedDate"`
	CommitSHA    string  `json:"commitSha"`
	CommitMsg    string  `json:"commitMsg"`
	Result       string  `json:"result"`
	DisplayTitle *string `json:"displayTitle"`
	Name         *string `json:"name"`
}

// DevLakeCICDDeployment represents a CICD deployment in DevLake format
type DevLakeCICDDeployment struct {
	ID                string                    `json:"id"`
	CreatedDate       *string                   `json:"createdDate"`
	StartedDate       string                    `json:"startedDate"`
	FinishedDate      string                    `json:"finishedDate"`
	Environment       string                    `json:"environment"`
	Result            string                    `json:"result"`
	DisplayTitle      *string                   `json:"displayTitle"`
	Name              *string                   `json:"name"`
	DeploymentCommits []DevLakeDeploymentCommit `json:"deploymentCommits"`
}

// DevLakeIntegration represents a DevLake-specific integration
type DevLakeIntegration struct {
	// Indicates if the integration is enabled
	enabled bool

	// Name of the integration
	name string

	// DevLake base URL
	baseURL string

	// DevLake project ID for webhook connections (global project)
	projectID string

	// HTTP client for making requests
	httpClient *http.Client

	// API timeout in seconds
	timeoutSeconds int

	// Team configurations for component-to-team mappings
	teams []config.TeamConfig
}

// NewDevLakeIntegration creates a new DevLake integration instance
func NewDevLakeIntegration(baseURL string, projectID string, enabled bool, timeoutSeconds int, teams []config.TeamConfig) *DevLakeIntegration {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30 // Default timeout
	}

	return &DevLakeIntegration{
		enabled:        enabled,
		name:           "devlake",
		baseURL:        baseURL,
		projectID:      projectID,
		httpClient:     &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
		timeoutSeconds: timeoutSeconds,
		teams:          teams,
	}
}

// GetTeamsForComponent returns all teams that contain the specified component
func (d *DevLakeIntegration) GetTeamsForComponent(component string) []config.TeamConfig {
	if component == "" || len(d.teams) == 0 {
		return nil
	}

	var matchingTeams []config.TeamConfig
	for _, team := range d.teams {
		for _, teamComponent := range team.ArgocdComponents {
			if teamComponent == component {
				matchingTeams = append(matchingTeams, team)
				break // Component found in this team, move to next team
			}
		}
	}
	return matchingTeams
}

// IsEnabled returns whether the integration is enabled
func (d *DevLakeIntegration) IsEnabled() bool {
	return d.enabled
}

// SendIncidentEvent sends a WebRCA incident event to DevLake
func (d *DevLakeIntegration) SendIncidentEvent(ctx context.Context, incident IncidentData, count int) error {
	if !d.enabled {
		return fmt.Errorf("devlake integration is disabled")
	}

	// Safety check: Only send Konflux incidents to DevLake
	if !d.isKonfluxIncident(incident) {
		logger.Debugf("Skipping non-Konflux incident %s - not sending to DevLake", incident.GetIncidentID())
		return nil
	}

	// Check if incident is resolved (both "resolved" and "closed" are treated the same)
	webrcaStatus := incident.GetStatus()
	isResolved := strings.ToLower(webrcaStatus) == "resolved" || strings.ToLower(webrcaStatus) == "closed"

	// Map WebRCA status to DevLake status (matching bash script logic exactly)
	devlakeStatus, originalStatus := d.mapToDevLakeStatus(webrcaStatus, isResolved)

	// Use the actual created date from WebRCA
	createdDate := d.FormatDevLakeDate(incident.GetCreatedAt())

	// Debug logging for date formatting
	logger.Debugf("Formatted dates - CreatedDate: %s", createdDate)

	// Debug logging to understand the field values
	logger.Debugf("Incident %s - WebRCA Status: %s, ResolvedAt: %v, IsResolved: %v",
		incident.GetIncidentID(),
		webrcaStatus,
		incident.GetResolvedAt(),
		isResolved)

	// Create DevLake issue payload following the bash script format
	devlakeIssue := &DevLakeIssue{
		IssueKey:       incident.GetIncidentID(),
		Title:          incident.GetSummary(),
		Description:    incident.GetDescription(),
		Type:           "INCIDENT",
		Status:         devlakeStatus,
		OriginalStatus: originalStatus, // Matches bash script logic
		CreatedDate:    createdDate,
		URL:            fmt.Sprintf("https://web-rca.devshift.net/incident/%s", incident.GetIncidentID()),
		Component:      d.getComponentFromProducts(incident.GetProducts()),
	}

	// Only add resolution date if incident is resolved (matching bash script logic exactly)
	if isResolved {
		logger.Debugf("Incident %s is resolved/closed, setting ResolutionDate", incident.GetIncidentID())
		// Use actual resolution time if available, otherwise fall back to updated time
		if resolvedAt := incident.GetResolvedAt(); resolvedAt != nil && !resolvedAt.IsZero() {
			devlakeIssue.ResolutionDate = d.FormatDevLakeDate(*resolvedAt)
			logger.Debugf("Using resolved_at for ResolutionDate: %s", devlakeIssue.ResolutionDate)
		} else {
			// Fallback to updated time if no resolution time available
			updatedAt := incident.GetUpdatedAt()
			if !updatedAt.IsZero() {
				devlakeIssue.ResolutionDate = d.FormatDevLakeDate(updatedAt)
				logger.Debugf("Using updated_at for ResolutionDate (resolved_at is nil or zero): %s", devlakeIssue.ResolutionDate)
			} else {
				// If both resolved_at and updated_at are zero, don't set ResolutionDate
				logger.Warnf("Both resolved_at and updated_at are zero for incident %s, not setting ResolutionDate", incident.GetIncidentID())
			}
		}
		logger.Debugf("Formatted ResolutionDate: %s", devlakeIssue.ResolutionDate)
	} else {
		logger.Debugf("Incident %s is not resolved/closed (status: %s), not setting ResolutionDate", incident.GetIncidentID(), webrcaStatus)
	}

	// Get DevLake token from environment
	token, err := d.getDevLakeToken()
	if err != nil {
		return fmt.Errorf("failed to get DevLake token: %w", err)
	}

	// Send HTTP POST to DevLake
	url := fmt.Sprintf("%s/api/rest/plugins/webhook/connections/%s/issues", d.baseURL, d.projectID)

	// Convert issue to JSON
	payload, err := json.Marshal(devlakeIssue)
	if err != nil {
		return fmt.Errorf("failed to marshal DevLake issue: %w", err)
	}

	logger.Debugf("DevLake API URL: %s", url)
	logger.Debugf("DevLake payload: %s", string(payload))

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create DevLake request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// Send request
	client := &http.Client{Timeout: time.Duration(d.timeoutSeconds) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to DevLake: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// Read response body for error details
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("DevLake API returned error status %d (failed to read response body: %v)", resp.StatusCode, err)
		}
		return fmt.Errorf("DevLake API returned error status %d: %s", resp.StatusCode, string(body))
	}

	logger.Debugf("DevLake incident sent successfully: %s (Status: %s)", incident.GetSummary(), incident.GetStatus())
	logger.Debugf("DevLake incident payload: %+v", devlakeIssue)

	return nil
}

// CloseIncident closes an incident in DevLake
func (d *DevLakeIntegration) CloseIncident(ctx context.Context, incidentID string) error {
	// Get DevLake token from environment
	token, err := d.getDevLakeToken()
	if err != nil {
		return fmt.Errorf("failed to get DevLake token: %w", err)
	}

	// Send HTTP POST to DevLake close endpoint
	url := fmt.Sprintf("%s/api/rest/plugins/webhook/connections/%s/issue/%s/close", d.baseURL, d.projectID, incidentID)

	logger.Debugf("DevLake close API URL: %s", url)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create DevLake close request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// Send request
	client := &http.Client{Timeout: time.Duration(d.timeoutSeconds) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send close request to DevLake: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// Read response body for error details
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("DevLake close API returned error status %d (failed to read response body: %v)", resp.StatusCode, err)
		}
		return fmt.Errorf("DevLake close API returned error status %d: %s", resp.StatusCode, string(body))
	}

	logger.Debugf("DevLake incident closed successfully: %s", incidentID)
	return nil
}

// extractComponentFromDisplayTitle extracts component name from DisplayTitle
// Supports both old and new formats:
// Old: "Production Deployment component: {component}, revision {revision} ({date})"
// New: "ArgoCD Deployment | Component: {component} | Cluster: {cluster} | Environment: {env} | Revision: {revision} | Commits: {count} | Status: {result} | Deployed: {timestamp}"
func (d *DevLakeIntegration) extractComponentFromDisplayTitle(displayTitle *string) string {
	if displayTitle == nil || *displayTitle == "" {
		return ""
	}

	// Try new format first: "Component: {component} |"
	prefix := "Component: "
	idx := strings.Index(*displayTitle, prefix)
	if idx != -1 {
		// Extract component name (everything after "Component: " until the next " |")
		start := idx + len(prefix)
		end := strings.Index((*displayTitle)[start:], " |")
		if end != -1 {
			return strings.TrimSpace((*displayTitle)[start : start+end])
		}
		// If no " |" found, try until end of string
		return strings.TrimSpace((*displayTitle)[start:])
	}

	// Fallback to old format: "component: {component},"
	prefix = "component: "
	idx = strings.Index(*displayTitle, prefix)
	if idx != -1 {
		start := idx + len(prefix)
		end := strings.Index((*displayTitle)[start:], ",")
		if end != -1 {
			return strings.TrimSpace((*displayTitle)[start : start+end])
		}
	}

	return ""
}

// sendDeploymentToProject sends a deployment to a specific DevLake project
func (d *DevLakeIntegration) sendDeploymentToProject(ctx context.Context, deployment DevLakeCICDDeployment, projectID string, projectName string) error {
	// Get DevLake token from environment
	token, err := d.getDevLakeToken()
	if err != nil {
		return fmt.Errorf("failed to get DevLake token: %w", err)
	}

	// Send HTTP POST to DevLake deployments endpoint
	url := fmt.Sprintf("%s/api/rest/plugins/webhook/connections/%s/deployments", d.baseURL, projectID)

	// Convert deployment to JSON
	payload, err := json.Marshal(deployment)
	if err != nil {
		return fmt.Errorf("failed to marshal DevLake deployment: %w", err)
	}

	logger.Debugf("DevLake deployment API URL: %s (project: %s)", url, projectName)
	logger.Debugf("DevLake deployment payload: %s", string(payload))

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create DevLake deployment request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// Send request
	client := &http.Client{Timeout: time.Duration(d.timeoutSeconds) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send deployment request to DevLake: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// Read response body for error details
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("DevLake deployment API returned error status %d (failed to read response body: %v)", resp.StatusCode, err)
		}
		return fmt.Errorf("DevLake deployment API returned error status %d: %s", resp.StatusCode, string(body))
	}

	logger.Infof("DevLake deployment sent successfully to %s: %s (ID: %s)", projectName, *deployment.DisplayTitle, deployment.ID)
	return nil
}

// SendDeploymentEvent sends an ArgoCD deployment event to DevLake
// It sends to the global project and all team projects where the component belongs
func (d *DevLakeIntegration) SendDeploymentEvent(ctx context.Context, deployment DevLakeCICDDeployment) error {
	if !d.enabled {
		return fmt.Errorf("devlake integration is disabled")
	}

	// Extract component name from DisplayTitle
	component := d.extractComponentFromDisplayTitle(deployment.DisplayTitle)

	// Get teams for this component
	teams := d.GetTeamsForComponent(component)

	// Track errors per project
	var errors []string
	successCount := 0

	// Always send to global project first
	if err := d.sendDeploymentToProject(ctx, deployment, d.projectID, "global"); err != nil {
		logger.Errorf("Failed to send deployment to global project: %v", err)
		errors = append(errors, fmt.Sprintf("global project: %v", err))
	} else {
		successCount++
	}

	// Send to all team projects where component belongs
	for _, team := range teams {
		if err := d.sendDeploymentToProject(ctx, deployment, team.ProjectID, team.Name); err != nil {
			logger.Errorf("Failed to send deployment to team project %s (ID: %s): %v", team.Name, team.ProjectID, err)
			errors = append(errors, fmt.Sprintf("team %s (project %s): %v", team.Name, team.ProjectID, err))
		} else {
			successCount++
			logger.Debugf("Deployment sent to team project %s (ID: %s) for component %s", team.Name, team.ProjectID, component)
		}
	}

	// Log summary
	totalProjects := 1 + len(teams) // global + teams
	if len(errors) > 0 {
		logger.Warnf("Deployment sent to %d/%d projects successfully. Errors: %v", successCount, totalProjects, errors)
		// Return error only if all projects failed
		if successCount == 0 {
			return fmt.Errorf("failed to send deployment to all projects: %v", errors)
		}
	} else {
		logger.Infof("Deployment sent successfully to all %d projects (global + %d team(s))", totalProjects, len(teams))
	}

	return nil
}

// getDevLakeToken returns the DevLake webhook token from environment variable
func (d *DevLakeIntegration) getDevLakeToken() (string, error) {
	token := os.Getenv("DEVLAKE_WEBHOOK_TOKEN")
	if token == "" {
		return "", fmt.Errorf("DEVLAKE_WEBHOOK_TOKEN environment variable is not set")
	}
	return token, nil
}

const (
	// DevLake date format - pre-defined for better performance
	devLakeDateFormat = "2006-01-02T15:04:05+00:00"
)

// FormatDevLakeDate formats time to DevLake required format: 2020-01-01T12:00:00+00:00
func (d *DevLakeIntegration) FormatDevLakeDate(t time.Time) string {
	// Check for zero time to prevent invalid datetime values
	if t.IsZero() {
		logger.Warnf("Attempted to format zero time, returning empty string")
		return ""
	}
	return t.UTC().Format(devLakeDateFormat)
}

// mapToDevLakeStatus maps WebRCA status to DevLake status format
func (d *DevLakeIntegration) mapToDevLakeStatus(webrcaStatus string, isResolved bool) (string, string) {
	if isResolved {
		// Both "resolved" and "closed" are treated as resolved in DevLake
		return "DONE", strings.ToLower(webrcaStatus)
	}
	return "TODO", "open"
}

// isKonfluxIncident checks if the incident is related to Konflux product
func (d *DevLakeIntegration) isKonfluxIncident(incident IncidentData) bool {
	products := incident.GetProducts()
	for _, product := range products {
		if product == "konflux" {
			return true
		}
	}
	return false
}

// getComponentFromProducts extracts the component from incident products
// For now, we only process Konflux incidents, so we return "konflux"
// In the future, this could be expanded to handle multiple products
func (d *DevLakeIntegration) getComponentFromProducts(products []string) string {
	// Check if any of the products is "konflux"
	for _, product := range products {
		if product == "konflux" {
			return "konflux"
		}
	}

	// If no konflux product found, return the first product or "unknown"
	if len(products) > 0 {
		return products[0]
	}

	return "unknown"
}
