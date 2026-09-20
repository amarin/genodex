package models

// Person — персона. Все поля, кроме id, опциональны. Private — приватность
// персональных данных (решение #24).
type Person struct {
	ID        ID
	Gender    PersonGender
	Names     []PersonName
	Estates   []TextRef
	Titles    []TextRef
	Nicknames []TextRef
	Notes     []TextRef
	Sources   []SourceLink
	Private   bool
}

// EntityType возвращает тип сущности.
func (p *Person) EntityType() Type { return TypePerson }
