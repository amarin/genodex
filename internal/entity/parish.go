package entity

// Parish сущность прихода
type Parish struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	ChurchID string            `json:"church_id,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Type возвращает тип сущности
func (p *Parish) Type() EntityType {
	return TypeParish
}
