package transport

import "github.com/amarin/genodex/internal/models"

// SurnameCreate — тело POST /api/surnames и аргументы тула surname_create.
// Идентификатор генерирует сценарий.
type SurnameCreate struct {
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s SurnameCreate) Model() models.Surname {
	return models.Surname{
		Canonical: s.Canonical,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}

// SurnameUpdate — тело PUT /api/surnames/{id} и аргументы тула surname_update:
// полная замена canonical/variants/items/notes.
type SurnameUpdate struct {
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s SurnameUpdate) Model() models.Surname {
	return models.Surname{
		Canonical: s.Canonical,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}
