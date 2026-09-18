package entity

// District сущность уезда
type District struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Governorate string            `json:"governorate_id"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Type возвращает тип сущности
func (d *District) Type() EntityType {
	return TypeDistrict
}
