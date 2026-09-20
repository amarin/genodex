package models

// Surname — словарная запись фамилии.
type Surname struct {
	ID        ID
	Canonical string
	Variants  []TextRef
	Items     []TextRef
	Notes     []TextRef
}

// EntityType возвращает тип сущности.
func (s *Surname) EntityType() Type { return TypeSurname }
