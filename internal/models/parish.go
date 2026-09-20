package models

// Parish — приход (внутренний тип сценариев).
// JSON-теги совпадают с internal/entity, чтобы сохранить внешние контракты.
type Parish struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	ChurchID string            `json:"church_id,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}
