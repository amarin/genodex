package transport

import "github.com/amarin/genodex/internal/models"

// Parish — контракт прихода (GET /api/parishes, MCP-тул parish_list). Church —
// необязательная ссылка (текст или ссылка на церковь); Since/Until — период
// действия прихода (структурированная дата, см. fact_date.go).
type Parish struct {
	ID          models.ID    `json:"id"`
	Name        string       `json:"name"`
	Church      *TextRef     `json:"church,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Since       *FactDate    `json:"since,omitempty"`
	Until       *FactDate    `json:"until,omitempty"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
}

// ParishFromModel конвертирует запись в контракт.
func ParishFromModel(p models.Parish) Parish {
	return Parish{
		ID:          p.ID,
		Name:        p.Name,
		Church:      TextRefFromModelPtr(p.Church),
		Settlements: TextRefsFromModel(p.Settlements),
		Since:       FactDateFromModel(p.Since),
		Until:       FactDateFromModel(p.Until),
		Notes:       TextRefsFromModel(p.Notes),
		Sources:     SourceLinksFromModel(p.Sources),
	}
}

// ParishesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func ParishesFromModels(ps []models.Parish) []Parish {
	out := make([]Parish, 0, len(ps))
	for _, p := range ps {
		out = append(out, ParishFromModel(p))
	}

	return out
}
