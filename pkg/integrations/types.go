package integrations

import "time"

// IncidentData represents the data needed for DevLake integration
type IncidentData interface {
	GetIncidentID() string
	GetSummary() string
	GetDescription() string
	GetStatus() string
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
	GetResolvedAt() *time.Time
	GetProducts() []string
}
