package models

// Event — событие жизненного факта (внутренний тип сценариев).
// JSON-теги совпадают с internal/entity, чтобы сохранить внешние контракты.
type Event struct {
	ID        string            `json:"id"`
	EventType string            `json:"type"`
	Date      string            `json:"date,omitempty"`
	Place     string            `json:"place,omitempty"`
	PersonIDs []string          `json:"person_ids"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}
