package transport

import "github.com/amarin/genodex/internal/models"

// PersonCreate — тело POST /api/people и аргументы тула person_create.
// Идентификатор генерирует сценарий.
type PersonCreate struct {
	Gender    string       `json:"gender,omitempty"`
	Names     []PersonName `json:"names"`
	Estates   []TextRef    `json:"estates"`
	Titles    []TextRef    `json:"titles"`
	Nicknames []TextRef    `json:"nicknames"`
	Notes     []TextRef    `json:"notes"`
	Sources   []SourceLink `json:"sources"`
	Private   bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (p PersonCreate) Model() models.Person {
	return models.Person{
		Gender:    models.PersonGender(p.Gender),
		Names:     PersonNamesToModel(p.Names),
		Estates:   TextRefsToModel(p.Estates),
		Titles:    TextRefsToModel(p.Titles),
		Nicknames: TextRefsToModel(p.Nicknames),
		Notes:     TextRefsToModel(p.Notes),
		Sources:   SourceLinksToModel(p.Sources),
		Private:   p.Private,
	}
}

// PersonUpdate — тело PUT /api/people/{id} и аргументы тула person_update:
// полная замена gender/names/estates/titles/nicknames/notes/sources/private.
type PersonUpdate struct {
	Gender    string       `json:"gender,omitempty"`
	Names     []PersonName `json:"names"`
	Estates   []TextRef    `json:"estates"`
	Titles    []TextRef    `json:"titles"`
	Nicknames []TextRef    `json:"nicknames"`
	Notes     []TextRef    `json:"notes"`
	Sources   []SourceLink `json:"sources"`
	Private   bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (p PersonUpdate) Model() models.Person {
	return models.Person{
		Gender:    models.PersonGender(p.Gender),
		Names:     PersonNamesToModel(p.Names),
		Estates:   TextRefsToModel(p.Estates),
		Titles:    TextRefsToModel(p.Titles),
		Nicknames: TextRefsToModel(p.Nicknames),
		Notes:     TextRefsToModel(p.Notes),
		Sources:   SourceLinksToModel(p.Sources),
		Private:   p.Private,
	}
}
