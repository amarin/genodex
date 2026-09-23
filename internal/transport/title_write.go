package transport

import "github.com/amarin/genodex/internal/models"

// TitleCreate — тело POST /api/titles и аргументы тула title_create.
// Идентификатор генерирует сценарий.
type TitleCreate struct {
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s TitleCreate) Model() models.Title {
	return models.Title{
		Canonical: s.Canonical,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}

// TitleUpdate — тело PUT /api/titles/{id} и аргументы тула title_update:
// полная замена canonical/variants/items/notes.
type TitleUpdate struct {
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s TitleUpdate) Model() models.Title {
	return models.Title{
		Canonical: s.Canonical,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}
