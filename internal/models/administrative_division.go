package models

// AdministrativeDivisionType — тип единицы административного деления.
type AdministrativeDivisionType string

// AdministrativeDivision — единица административного деления
// (внутренний тип сценариев).
// JSON-теги совпадают с internal/entity, чтобы сохранить внешние контракты.
type AdministrativeDivision struct {
	ID                         string                     `json:"id"`
	Name                       string                     `json:"name"`
	AdministrativeDivisionType AdministrativeDivisionType `json:"type"`
	Metadata                   map[string]string          `json:"metadata,omitempty"`
}
