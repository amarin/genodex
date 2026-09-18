package entity

// Marriage сущность брака
type Marriage struct {
	ID       string            `json:"id"`
	GroomID  string            `json:"groom_id"`
	BrideID  string            `json:"bride_id"`
	Date     string            `json:"date,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Type возвращает тип сущности
func (m *Marriage) Type() EntityType {
	return TypeMarriage
}
