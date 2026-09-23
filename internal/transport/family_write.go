package transport

import "github.com/amarin/genodex/internal/models"

// FamilyCreate — тело POST /api/families и аргументы тула family_create.
// Идентификатор генерирует сценарий.
type FamilyCreate struct {
	Name    string       `json:"name"`
	Members []TextRef    `json:"members"`
	Notes   []TextRef    `json:"notes"`
	Sources []SourceLink `json:"sources"`
	Private bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (f FamilyCreate) Model() models.Family {
	return models.Family{
		Name:    f.Name,
		Members: TextRefsToModel(f.Members),
		Notes:   TextRefsToModel(f.Notes),
		Sources: SourceLinksToModel(f.Sources),
		Private: f.Private,
	}
}

// FamilyUpdate — тело PUT /api/families/{id} и аргументы тула
// family_update: полная замена name/members/notes/sources/private.
type FamilyUpdate struct {
	Name    string       `json:"name"`
	Members []TextRef    `json:"members"`
	Notes   []TextRef    `json:"notes"`
	Sources []SourceLink `json:"sources"`
	Private bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (f FamilyUpdate) Model() models.Family {
	return models.Family{
		Name:    f.Name,
		Members: TextRefsToModel(f.Members),
		Notes:   TextRefsToModel(f.Notes),
		Sources: SourceLinksToModel(f.Sources),
		Private: f.Private,
	}
}
