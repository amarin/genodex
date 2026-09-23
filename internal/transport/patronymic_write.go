package transport

import "github.com/amarin/genodex/internal/models"

// PatronymicCreate — тело POST /api/patronymics и аргументы тула patronymic_create.
// Идентификатор генерирует сценарий.
type PatronymicCreate struct {
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s PatronymicCreate) Model() models.Patronymic {
	return models.Patronymic{
		Canonical: s.Canonical,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}

// PatronymicUpdate — тело PUT /api/patronymics/{id} и аргументы тула patronymic_update:
// полная замена canonical/variants/items/notes.
type PatronymicUpdate struct {
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s PatronymicUpdate) Model() models.Patronymic {
	return models.Patronymic{
		Canonical: s.Canonical,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}
