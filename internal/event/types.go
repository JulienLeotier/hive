package event

import "time"

// Event represents a system event persisted to the event log.
type Event struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Source    string    `json:"source"`
	Payload   string    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

// Subscriber is a callback invoked when a matching event is published.
type Subscriber func(Event)
