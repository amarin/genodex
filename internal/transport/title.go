package transport

import "github.com/amarin/genodex/internal/models"

// Title — контракт словарной записи фамилии (GET /api/titles, MCP-тул title_list).
type Title struct {
	ID        models.ID `json:"id"`
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// TitleFromModel конвертирует запись в контракт.
func TitleFromModel(s models.Title) Title {
	return Title{
		ID:        s.ID,
		Canonical: s.Canonical,
		Variants:  TextRefsFromModel(s.Variants),
		Items:     TextRefsFromModel(s.Items),
		Notes:     TextRefsFromModel(s.Notes),
	}
}

// TitlesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func TitlesFromModels(ss []models.Title) []Title {
	out := make([]Title, 0, len(ss))
	for _, s := range ss {
		out = append(out, TitleFromModel(s))
	}

	return out
}
