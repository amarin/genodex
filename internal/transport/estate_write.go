package transport

import "github.com/amarin/genodex/internal/models"

// EstateCreate — тело POST /api/estates и аргументы тула estate_create.
// Идентификатор генерирует сценарий.
type EstateCreate struct {
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s EstateCreate) Model() models.Estate {
	return models.Estate{
		Canonical: s.Canonical,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}

// EstateUpdate — тело PUT /api/estates/{id} и аргументы тула estate_update:
// полная замена canonical/variants/items/notes.
type EstateUpdate struct {
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s EstateUpdate) Model() models.Estate {
	return models.Estate{
		Canonical: s.Canonical,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}
