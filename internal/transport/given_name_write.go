package transport

import "github.com/amarin/genodex/internal/models"

// GivenNameCreate — тело POST /api/given-names и аргументы тула given_name_create.
// Идентификатор генерирует сценарий.
type GivenNameCreate struct {
	Canonical string            `json:"canonical"`
	Gender    models.NameGender `json:"gender"`
	Variants  []TextRef         `json:"variants"`
	Items     []TextRef         `json:"items"`
	Notes     []TextRef         `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s GivenNameCreate) Model() models.GivenName {
	return models.GivenName{
		Canonical: s.Canonical,
		Gender:    s.Gender,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}

// GivenNameUpdate — тело PUT /api/given-names/{id} и аргументы тула given_name_update:
// полная замена canonical/variants/items/notes.
type GivenNameUpdate struct {
	Canonical string            `json:"canonical"`
	Gender    models.NameGender `json:"gender"`
	Variants  []TextRef         `json:"variants"`
	Items     []TextRef         `json:"items"`
	Notes     []TextRef         `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s GivenNameUpdate) Model() models.GivenName {
	return models.GivenName{
		Canonical: s.Canonical,
		Gender:    s.Gender,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}
