package entity

// AdministrativeDivision задаёт структуру данных административного деления
type AdministrativeDivision struct {
	ID                         string                     `json:"id"`
	Name                       string                     `json:"name"`
	AdministrativeDivisionType AdministrativeDivisionType `json:"type"`
	Metadata                   map[string]string          `json:"metadata,omitempty"`
}

// Type возвращает тип сущности
func (g *AdministrativeDivision) Type() EntityType {
	return TypeAdministrativeDivision
}
