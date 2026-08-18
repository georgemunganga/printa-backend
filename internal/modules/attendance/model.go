package attendance

import (
	"time"

	"github.com/google/uuid"
)

// EventType describes a staff member's store attendance transition.
type EventType string

const (
	EventClockIn  EventType = "CLOCK_IN"
	EventClockOut EventType = "CLOCK_OUT"
)

// AttendanceEvent is an immutable record of a successful store clock transition.
type AttendanceEvent struct {
	ID         uuid.UUID  `json:"id"`
	StoreID    uuid.UUID  `json:"store_id"`
	UserID     uuid.UUID  `json:"user_id"`
	EventType  EventType  `json:"event_type"`
	OccurredAt time.Time  `json:"occurred_at"`
	CreatedBy  *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// SetPINRequest is restricted to an owning vendor, manager, or administrator.
type SetPINRequest struct {
	PIN string `json:"pin"`
}

// ClockRequest verifies the selected assigned staff member's PIN server-side.
type ClockRequest struct {
	UserID string `json:"user_id"`
	PIN    string `json:"pin"`
}

// ClockResponse reports the persisted attendance transition without exposing sensitive hashes.
type ClockResponse struct {
	Event      AttendanceEvent `json:"event"`
	NextAction EventType       `json:"next_action"`
}

func isValidPIN(pin string) bool {
	if len(pin) < 4 || len(pin) > 6 {
		return false
	}
	for _, char := range pin {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
