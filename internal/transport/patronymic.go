package transport

import "github.com/amarin/genodex/internal/models"

// Patronymic — контракт словарной записи отчества (GET /api/patronymics, MCP-тул patronymic_list).
type Patronymic struct {
	ID        models.ID `json:"id"`
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// PatronymicFromModel конвертирует запись в контракт.
func PatronymicFromModel(s models.Patronymic) Patronymic {
	return Patronymic{
		ID:        s.ID,
		Canonical: s.Canonical,
		Variants:  TextRefsFromModel(s.Variants),
		Items:     TextRefsFromModel(s.Items),
		Notes:     TextRefsFromModel(s.Notes),
	}
}

// PatronymicsFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func PatronymicsFromModels(ss []models.Patronymic) []Patronymic {
	out := make([]Patronymic, 0, len(ss))
	for _, s := range ss {
		out = append(out, PatronymicFromModel(s))
	}

	return out
}
