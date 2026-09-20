package models

// Church — церковь.
type Church struct {
	ID          ID
	Name        string
	Parish      *TextRef
	Settlements []TextRef
	Variants    []string
	Notes       []TextRef
	Sources     []SourceLink
}

// EntityType возвращает тип сущности.
func (c *Church) EntityType() Type { return TypeChurch }
