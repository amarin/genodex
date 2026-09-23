package transport

import "github.com/amarin/genodex/internal/models"

// Surname — контракт словарной записи фамилии (GET /api/surnames, MCP-тул surname_list).
type Surname struct {
	ID        models.ID `json:"id"`
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// SurnameFromModel конвертирует запись в контракт.
func SurnameFromModel(s models.Surname) Surname {
	return Surname{
		ID:        s.ID,
		Canonical: s.Canonical,
		Variants:  TextRefsFromModel(s.Variants),
		Items:     TextRefsFromModel(s.Items),
		Notes:     TextRefsFromModel(s.Notes),
	}
}

// SurnamesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func SurnamesFromModels(ss []models.Surname) []Surname {
	out := make([]Surname, 0, len(ss))
	for _, s := range ss {
		out = append(out, SurnameFromModel(s))
	}

	return out
}
