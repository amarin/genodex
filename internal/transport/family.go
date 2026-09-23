package transport

import "github.com/amarin/genodex/internal/models"

// Family — контракт рода/линии (GET /api/families, MCP-тул family_list).
// Sources редактируется с рождения контракта (сущность заведена уже после
// подпроекта 5, см. internal/transport/source_link.go).
type Family struct {
	ID      models.ID    `json:"id"`
	Name    string       `json:"name"`
	Members []TextRef    `json:"members"`
	Notes   []TextRef    `json:"notes"`
	Sources []SourceLink `json:"sources"`
	Private bool         `json:"private"`
}

// FamilyFromModel конвертирует запись в контракт.
func FamilyFromModel(f models.Family) Family {
	return Family{
		ID:      f.ID,
		Name:    f.Name,
		Members: TextRefsFromModel(f.Members),
		Notes:   TextRefsFromModel(f.Notes),
		Sources: SourceLinksFromModel(f.Sources),
		Private: f.Private,
	}
}

// FamiliesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func FamiliesFromModels(fs []models.Family) []Family {
	out := make([]Family, 0, len(fs))
	for _, f := range fs {
		out = append(out, FamilyFromModel(f))
	}

	return out
}
