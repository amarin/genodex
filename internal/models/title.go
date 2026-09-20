package models

// Title — словарная запись звания/титула.
type Title struct {
	ID        ID
	Canonical string
	Variants  []TextRef
	Items     []TextRef
	Notes     []TextRef
}

// EntityType возвращает тип сущности.
func (t *Title) EntityType() Type { return TypeTitle }
