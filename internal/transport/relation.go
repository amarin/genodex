package transport

import "github.com/amarin/genodex/internal/models"

// Relation — контракт ребра графа родства (GET /api/relations, MCP-тул
// relation_list). Sources редактируется с рождения контракта (сущность
// заведена уже после подпроекта 5). Первая сущность программы с двумя
// строгими ссылками на один и тот же тип (PersonA/PersonB → Person, см.
// docs/data-model/entity-write.md §3.8).
type Relation struct {
	ID      models.ID    `json:"id"`
	Kind    string       `json:"kind"`
	RelType string       `json:"rel_type,omitempty"`
	PersonA models.ID    `json:"person_a"`
	PersonB models.ID    `json:"person_b"`
	Since   *FactDate    `json:"since,omitempty"`
	Until   *FactDate    `json:"until,omitempty"`
	Sources []SourceLink `json:"sources"`
	Notes   []TextRef    `json:"notes"`
	Private bool         `json:"private"`
}

// RelationFromModel конвертирует запись в контракт.
func RelationFromModel(r models.Relation) Relation {
	return Relation{
		ID:      r.ID,
		Kind:    string(r.Kind),
		RelType: string(r.RelType),
		PersonA: r.PersonA,
		PersonB: r.PersonB,
		Since:   FactDateFromModel(r.Since),
		Until:   FactDateFromModel(r.Until),
		Sources: SourceLinksFromModel(r.Sources),
		Notes:   TextRefsFromModel(r.Notes),
		Private: r.Private,
	}
}

// RelationsFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func RelationsFromModels(rs []models.Relation) []Relation {
	out := make([]Relation, 0, len(rs))
	for _, r := range rs {
		out = append(out, RelationFromModel(r))
	}

	return out
}
