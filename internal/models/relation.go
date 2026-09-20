package models

// Relation — ребро графа родства между двумя персонами. RelType задаётся
// только для kind=associate (решение #23). Private — приватность (решение #24).
type Relation struct {
	ID      ID
	Kind    RelationKind
	RelType RelationType
	PersonA ID
	PersonB ID
	Since   *FactDate
	Until   *FactDate
	Sources []SourceLink
	Notes   []TextRef
	Private bool
}

// EntityType возвращает тип сущности.
func (r *Relation) EntityType() Type { return TypeRelation }
