package models

// Residence — проживание персоны в месте (AdministrativeDivision) с периодом.
type Residence struct {
	ID       ID
	PersonID ID
	PlaceID  ID
	Since    *FactDate
	Until    *FactDate
	Sources  []SourceLink
	Note     string
	Private  bool
}

// EntityType возвращает тип сущности.
func (r *Residence) EntityType() Type { return TypeResidence }
