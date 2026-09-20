package models

// GivenName — словарная запись имени; обязателен признак пола.
type GivenName struct {
	ID        ID
	Canonical string
	Gender    NameGender
	Variants  []TextRef
	Items     []TextRef
	Notes     []TextRef
}

// EntityType возвращает тип сущности.
func (g *GivenName) EntityType() Type { return TypeGivenName }
