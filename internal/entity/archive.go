package entity

// Archive сущность архива
type Archive struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Type возвращает тип сущности
func (a *Archive) Type() EntityType {
	return TypeArchive
}
