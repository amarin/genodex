package entity

// Case сущность дела
type Case struct {
	ID        string            `json:"id"`
	Number    int               `json:"number"`
	Inventory string            `json:"inventory_id"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Type возвращает тип сущности
func (c *Case) Type() EntityType {
	return TypeCase
}
