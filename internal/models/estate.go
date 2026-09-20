package models

// Estate — словарная запись сословия/общинного статуса.
type Estate struct {
	ID        ID
	Canonical string
	Variants  []TextRef
	Items     []TextRef
	Notes     []TextRef
}

// EntityType возвращает тип сущности.
func (e *Estate) EntityType() Type { return TypeEstate }
