package webrca

import "time"

// IncidentList represents the response structure from the WebRCA API.
// It contains paginated incident data with metadata about the response.
type IncidentList struct {
	Kind  string     `json:"kind"`
	Page  int        `json:"page"`
	Size  int        `json:"size"`
	Total int        `json:"total"`
	Items []Incident `json:"items"`
}

// Incident represents a single incident from the WebRCA system.
// It contains comprehensive details about incidents including metadata,
// participants, timeline, and resolution information for DORA metrics tracking.
type Incident struct {
	// ID is the unique identifier for this incident
	ID string `json:"id"`

	// Kind identifies the type of object - always "Incident"
	Kind string `json:"kind"`

	// Href provides the API endpoint URL for this specific incident
	Href string `json:"href"`

	// IncidentID is the human-readable incident identifier (e.g., "ITN-2025-00217")
	IncidentID string `json:"incident_id"`

	// Summary provides a brief description of the incident
	Summary string `json:"summary"`

	// Description contains detailed information about the incident
	Description string `json:"description"`

	// Products lists the affected products or services
	Products []string `json:"products"`

	// IncidentType categorizes the incident (e.g., "customer_facing", "internal")
	IncidentType string `json:"incident_type"`

	// Status indicates the current state of the incident
	// Possible values: "open", "investigating", "resolved", "closed"
	Status string `json:"status"`

	// Severity indicates the impact level of the incident
	// Typically numeric values like "1" (critical), "2" (high), "3" (medium), "4" (low)
	Severity string `json:"severity"`

	// SlackWorkspaceID identifies the Slack workspace used for coordination
	SlackWorkspaceID string `json:"slack_workspace_id"`

	// SlackChannelID identifies the specific Slack channel for this incident
	SlackChannelID string `json:"slack_channel_id"`

	// ExternalCoordination contains URLs to external coordination tools
	// (Slack channels, Google Meet links, etc.)
	ExternalCoordination []string `json:"external_coordination"`

	// Creator contains information about the user who created this incident
	Creator User `json:"creator"`

	// Participants lists all users involved in resolving this incident
	Participants []User `json:"participants"`

	// Timeline contains chronological events related to this incident
	Timeline []TimelineEvent `json:"timeline"`

	// CreatedAt indicates when the incident was first created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt indicates when the incident was last modified
	UpdatedAt time.Time `json:"updated_at"`

	// ResolvedAt indicates when the incident was resolved (if applicable)
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`

	// ClosedAt indicates when the incident was closed (if applicable)
	ClosedAt *time.Time `json:"closed_at,omitempty"`
}

// IsKonfluxIncident checks if this incident is related to Konflux product
func (i *Incident) IsKonfluxIncident() bool {
	for _, product := range i.Products {
		if product == "konflux" {
			return true
		}
	}
	return false
}

// IsResolved checks if the incident is in a resolved state
func (i *Incident) IsResolved() bool {
	return i.Status == "resolved" || i.Status == "closed"
}

// GetIncidentID returns the incident ID for integration purposes
func (i *Incident) GetIncidentID() string {
	return i.IncidentID
}

// GetSummary returns the incident summary for integration purposes
func (i *Incident) GetSummary() string {
	return i.Summary
}

// GetDescription returns the incident description for integration purposes
func (i *Incident) GetDescription() string {
	return i.Description
}

// GetStatus returns the incident status for integration purposes
func (i *Incident) GetStatus() string {
	return i.Status
}

// GetCreatedAt returns the incident creation time for integration purposes
func (i *Incident) GetCreatedAt() time.Time {
	return i.CreatedAt
}

// GetUpdatedAt returns the incident update time for integration purposes
func (i *Incident) GetUpdatedAt() time.Time {
	return i.UpdatedAt
}

// GetResolvedAt returns the incident resolution time for integration purposes
func (i *Incident) GetResolvedAt() *time.Time {
	return i.ResolvedAt
}

// GetProducts returns the incident products for integration purposes
func (i *Incident) GetProducts() []string {
	return i.Products
}

// User represents a user in the WebRCA system.
// It includes information about incident creators and participants.
type User struct {
	// ID is the unique identifier for this user
	ID string `json:"id"`

	// Kind identifies the type of object - always "User"
	Kind string `json:"kind"`

	// Href provides the API endpoint URL for this specific user
	Href string `json:"href"`

	// Name is the display name of the user
	Name string `json:"name"`

	// Email is the email address of the user
	Email string `json:"email"`

	// Username is the username/handle of the user
	Username string `json:"username"`

	// CreatedAt indicates when the user account was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt indicates when the user account was last modified
	UpdatedAt time.Time `json:"updated_at"`
}

// TimelineEvent represents a single event in the incident timeline.
// It tracks the progression of an incident from creation to resolution.
type TimelineEvent struct {
	// ID is the unique identifier for this timeline event
	ID string `json:"id"`

	// Kind identifies the type of object - always "TimelineEvent"
	Kind string `json:"kind"`

	// Href provides the API endpoint URL for this specific timeline event
	Href string `json:"href"`

	// EventType categorizes the type of event
	// Examples: "created", "status_changed", "participant_added", "comment_added"
	EventType string `json:"event_type"`

	// Description provides details about what happened in this event
	Description string `json:"description"`

	// Actor contains information about the user who performed this action
	Actor User `json:"actor"`

	// CreatedAt indicates when this timeline event occurred
	CreatedAt time.Time `json:"created_at"`
}
