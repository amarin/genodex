package entity

// Inventory сущность описи
type Inventory struct {
	ID       string            `json:"id"`
	Number   int               `json:"number"`
	Fund     string            `json:"fund_id"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Type возвращает тип сущности
func (i *Inventory) Type() EntityType {
	return TypeInventory
}
