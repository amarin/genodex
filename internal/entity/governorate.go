package entity

// Governorate сущность губернии
type Governorate struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Type возвращает тип сущности
func (g *Governorate) Type() EntityType {
	return TypeGovernorate
}
