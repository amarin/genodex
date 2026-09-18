package entity

// Fund сущность фонда
type Fund struct {
	ID       string            `json:"id"`
	Code     string            `json:"code"`
	Archive  string            `json:"archive_id"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Type возвращает тип сущности
func (f *Fund) Type() EntityType {
	return TypeFund
}
