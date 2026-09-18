package entity

// Source сущность источника
type Source struct {
	ID        string            `json:"id"`
	EventType string            `json:"type"`
	Title     string            `json:"title"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Type возвращает тип сущности
func (s *Source) Type() EntityType {
	return TypeSource
}
