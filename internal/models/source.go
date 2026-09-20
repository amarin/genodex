package models

// Source — источник доказательства (внутренний тип сценариев).
// JSON-теги совпадают с internal/entity, чтобы сохранить внешние контракты.
type Source struct {
	ID        string            `json:"id"`
	EventType string            `json:"type"`
	Title     string            `json:"title"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}
