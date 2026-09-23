package transport

import "github.com/amarin/genodex/internal/models"

// GivenName — контракт словарной записи имени (GET /api/given-names, MCP-тул given_name_list).
type GivenName struct {
	ID        models.ID         `json:"id"`
	Canonical string            `json:"canonical"`
	Gender    models.NameGender `json:"gender"`
	Variants  []TextRef         `json:"variants"`
	Items     []TextRef         `json:"items"`
	Notes     []TextRef         `json:"notes"`
}

// GivenNameFromModel конвертирует запись в контракт.
func GivenNameFromModel(s models.GivenName) GivenName {
	return GivenName{
		ID:        s.ID,
		Canonical: s.Canonical,
		Gender:    s.Gender,
		Variants:  TextRefsFromModel(s.Variants),
		Items:     TextRefsFromModel(s.Items),
		Notes:     TextRefsFromModel(s.Notes),
	}
}

// GivenNamesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func GivenNamesFromModels(ss []models.GivenName) []GivenName {
	out := make([]GivenName, 0, len(ss))
	for _, s := range ss {
		out = append(out, GivenNameFromModel(s))
	}

	return out
}
