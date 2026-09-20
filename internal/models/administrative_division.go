package models

// AdministrativeDivision — единая рекурсивная единица административного
// деления; тип совмещает уровень деления и вид населённого пункта (#17).
type AdministrativeDivision struct {
	ID         ID
	Name       string
	Type       AdminDivisionType
	ParentID   *ID
	Items      []TextRef
	Variants   []string
	Renames    []NamedPeriod
	Successors []TextRef
	Since      *FactDate
	Until      *FactDate
	Notes      []TextRef
	Sources    []SourceLink
}

// EntityType возвращает тип сущности.
func (a *AdministrativeDivision) EntityType() Type { return TypeAdministrativeDivision }
