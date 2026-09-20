package models

// RelationKind — вид ребра графа родства (решение #23).
type RelationKind string

const (
	RelationKindBlood     RelationKind = "blood"
	RelationKindMarriage  RelationKind = "marriage"
	RelationKindAdoption  RelationKind = "adoption"
	RelationKindAssociate RelationKind = "associate"
)
