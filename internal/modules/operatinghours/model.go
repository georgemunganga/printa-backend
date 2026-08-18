package operatinghours

// OperatingHour describes one weekday's published operating availability for a store.
type OperatingHour struct {
	DayOfWeek int    `json:"day_of_week"`
	IsOpen    bool   `json:"is_open"`
	OpensAt   string `json:"opens_at,omitempty"`
	ClosesAt  string `json:"closes_at,omitempty"`
}

// ReplaceRequest replaces a store's complete seven-day operating-hours schedule.
type ReplaceRequest struct {
	Hours []OperatingHour `json:"hours"`
}
