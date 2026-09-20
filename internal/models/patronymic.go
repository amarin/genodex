package models

// Patronymic — словарная запись отчества.
type Patronymic struct {
	ID        ID
	Canonical string
	Variants  []TextRef
	Items     []TextRef
	Notes     []TextRef
}

// EntityType возвращает тип сущности.
func (p *Patronymic) EntityType() Type { return TypePatronymic }
