package transport

import "github.com/amarin/genodex/internal/models"

// Estate — контракт словарной записи сословия (GET /api/estates, MCP-тул estate_list).
type Estate struct {
	ID        models.ID `json:"id"`
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// EstateFromModel конвертирует запись в контракт.
func EstateFromModel(s models.Estate) Estate {
	return Estate{
		ID:        s.ID,
		Canonical: s.Canonical,
		Variants:  TextRefsFromModel(s.Variants),
		Items:     TextRefsFromModel(s.Items),
		Notes:     TextRefsFromModel(s.Notes),
	}
}

// EstatesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func EstatesFromModels(ss []models.Estate) []Estate {
	out := make([]Estate, 0, len(ss))
	for _, s := range ss {
		out = append(out, EstateFromModel(s))
	}

	return out
}
