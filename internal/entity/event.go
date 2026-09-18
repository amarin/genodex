package entity

// Event сущность события
type Event struct {
	ID        string            `json:"id"`
	EventType string            `json:"type"`
	Date      string            `json:"date,omitempty"`
	Place     string            `json:"place,omitempty"`
	PersonIDs []string          `json:"person_ids"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Type возвращает тип сущности
func (e *Event) Type() EntityType {
	return TypeEvent
}
