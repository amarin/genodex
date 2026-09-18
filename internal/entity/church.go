package entity

// Church сущность церкви
type Church struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Type возвращает тип сущности
func (c *Church) Type() EntityType {
	return TypeChurch
}
