package models

// Family — род/линия: группирующая сущность.
type Family struct {
	ID      ID
	Name    string
	Members []TextRef
	Notes   []TextRef
	Sources []SourceLink
	Private bool
}

// EntityType возвращает тип сущности.
func (f *Family) EntityType() Type { return TypeFamily }
