package transport

import "github.com/amarin/genodex/internal/models"

// ChurchCreate — тело POST /api/churches и аргументы тула church_create.
// Идентификатор генерирует сценарий. Parish — {text, ref?, type?}: сама DTO
// round-trip'ит ref/type как есть (TextRef.Model), веб-форма v1 (web/src/
// ChurchForm.tsx) редактирует только text и не выставляет ref/type сама
// (docs/data-model/entity-write.md §4) — ограничение интерфейса, а не
// контракта.
type ChurchCreate struct {
	Name        string       `json:"name"`
	Parish      *TextRef     `json:"parish,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Variants    []string     `json:"variants"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
}

// Model возвращает доменную запись с пустым ID.
func (c ChurchCreate) Model() models.Church {
	return models.Church{
		Name:        c.Name,
		Parish:      c.Parish.ModelPtr(),
		Settlements: TextRefsToModel(c.Settlements),
		Variants:    c.Variants,
		Notes:       TextRefsToModel(c.Notes),
		Sources:     SourceLinksToModel(c.Sources),
	}
}

// ChurchUpdate — тело PUT /api/churches/{id} и аргументы тула church_update:
// полная замена name/parish/settlements/variants/notes/sources.
type ChurchUpdate struct {
	Name        string       `json:"name"`
	Parish      *TextRef     `json:"parish,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Variants    []string     `json:"variants"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
}

// Model возвращает доменную запись с пустым ID.
func (c ChurchUpdate) Model() models.Church {
	return models.Church{
		Name:        c.Name,
		Parish:      c.Parish.ModelPtr(),
		Settlements: TextRefsToModel(c.Settlements),
		Variants:    c.Variants,
		Notes:       TextRefsToModel(c.Notes),
		Sources:     SourceLinksToModel(c.Sources),
	}
}
