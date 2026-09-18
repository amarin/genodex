package entity

// Settlement сущность населённого пункта
type Settlement struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Type возвращает тип сущности
func (s *Settlement) Type() EntityType {
	return TypeSettlement
}
