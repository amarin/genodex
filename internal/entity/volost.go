package entity

// Volost сущность волости
type Volost struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	District string            `json:"district_id"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Type возвращает тип сущности
func (v *Volost) Type() EntityType {
	return TypeVolost
}
