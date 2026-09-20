package models

// Archive — архив (внутренний тип сценариев).
// JSON-теги совпадают с internal/entity, чтобы сохранить внешние контракты.
type Archive struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata,omitempty"`
}
