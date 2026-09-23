package transport

import "github.com/amarin/genodex/internal/models"

// ChurchCreate — тело POST /api/churches и аргументы тула church_create.
// Идентификатор генерирует сценарий. Parish редактируется только текстом (v1,
// docs/data-model/entity-write.md §4) — элемент с уже заполненным ref через
// этот DTO не создать.
type ChurchCreate struct {
	Name        string    `json:"name"`
	Parish      *TextRef  `json:"parish,omitempty"`
	Settlements []TextRef `json:"settlements"`
	Variants    []string  `json:"variants"`
	Notes       []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (c ChurchCreate) Model() models.Church {
	return models.Church{
		Name:        c.Name,
		Parish:      c.Parish.ModelPtr(),
		Settlements: TextRefsToModel(c.Settlements),
		Variants:    c.Variants,
		Notes:       TextRefsToModel(c.Notes),
	}
}

// ChurchUpdate — тело PUT /api/churches/{id} и аргументы тула church_update:
// полная замена name/parish/settlements/variants/notes.
type ChurchUpdate struct {
	Name        string    `json:"name"`
	Parish      *TextRef  `json:"parish,omitempty"`
	Settlements []TextRef `json:"settlements"`
	Variants    []string  `json:"variants"`
	Notes       []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (c ChurchUpdate) Model() models.Church {
	return models.Church{
		Name:        c.Name,
		Parish:      c.Parish.ModelPtr(),
		Settlements: TextRefsToModel(c.Settlements),
		Variants:    c.Variants,
		Notes:       TextRefsToModel(c.Notes),
	}
}
