package agent

import "time"

// Agent represents a registered agent in the hive.
type Agent struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Config       string    `json:"config"`
	Capabilities string    `json:"capabilities"`
	HealthStatus string    `json:"health_status"`
	TrustLevel   string    `json:"trust_level"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
