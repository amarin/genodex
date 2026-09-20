package models

// Parish — приход.
type Parish struct {
	ID          ID
	Name        string
	Church      *TextRef
	Settlements []TextRef
	Since       *FactDate
	Until       *FactDate
	Notes       []TextRef
	Sources     []SourceLink
}

// EntityType возвращает тип сущности.
func (p *Parish) EntityType() Type { return TypeParish }
