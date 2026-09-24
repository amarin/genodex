package transport

import "github.com/amarin/genodex/internal/models"

// RelationCreate — тело POST /api/relations и аргументы тула
// relation_create. Идентификатор генерирует сценарий.
type RelationCreate struct {
	Kind    string       `json:"kind"`
	RelType string       `json:"rel_type,omitempty"`
	PersonA string       `json:"person_a"`
	PersonB string       `json:"person_b"`
	Since   *FactDate    `json:"since,omitempty"`
	Until   *FactDate    `json:"until,omitempty"`
	Sources []SourceLink `json:"sources"`
	Notes   []TextRef    `json:"notes"`
	Private bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (r RelationCreate) Model() models.Relation {
	return models.Relation{
		Kind:    models.RelationKind(r.Kind),
		RelType: models.RelationType(r.RelType),
		PersonA: models.ID(r.PersonA),
		PersonB: models.ID(r.PersonB),
		Since:   r.Since.Model(),
		Until:   r.Until.Model(),
		Sources: SourceLinksToModel(r.Sources),
		Notes:   TextRefsToModel(r.Notes),
		Private: r.Private,
	}
}

// RelationUpdate — тело PUT /api/relations/{id} и аргументы тула
// relation_update: полная замена kind/rel_type/person_a/person_b/since/
// until/sources/notes/private.
type RelationUpdate struct {
	Kind    string       `json:"kind"`
	RelType string       `json:"rel_type,omitempty"`
	PersonA string       `json:"person_a"`
	PersonB string       `json:"person_b"`
	Since   *FactDate    `json:"since,omitempty"`
	Until   *FactDate    `json:"until,omitempty"`
	Sources []SourceLink `json:"sources"`
	Notes   []TextRef    `json:"notes"`
	Private bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (r RelationUpdate) Model() models.Relation {
	return models.Relation{
		Kind:    models.RelationKind(r.Kind),
		RelType: models.RelationType(r.RelType),
		PersonA: models.ID(r.PersonA),
		PersonB: models.ID(r.PersonB),
		Since:   r.Since.Model(),
		Until:   r.Until.Model(),
		Sources: SourceLinksToModel(r.Sources),
		Notes:   TextRefsToModel(r.Notes),
		Private: r.Private,
	}
}
