package models

// RelationType — вид связи для kind=associate (нет родства: сосед, коллега,
// кум, свидетель, друг и т.п.). Значения свободные и расширяемые; не влияют на
// построение дерева родства.
type RelationType string

const (
	RelationTypeNeighbor  RelationType = "neighbor"
	RelationTypeColleague RelationType = "colleague"
	RelationTypeGodparent RelationType = "godparent"
	RelationTypeWitness   RelationType = "witness"
	RelationTypeFriend    RelationType = "friend"
)
