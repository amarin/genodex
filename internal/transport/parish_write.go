package transport

import "github.com/amarin/genodex/internal/models"

// ParishCreate — тело POST /api/parishes и аргументы тула parish_create.
// Идентификатор генерирует сценарий. Church редактируется только текстом (v1).
type ParishCreate struct {
	Name        string    `json:"name"`
	Church      *TextRef  `json:"church,omitempty"`
	Settlements []TextRef `json:"settlements"`
	Since       *FactDate `json:"since,omitempty"`
	Until       *FactDate `json:"until,omitempty"`
	Notes       []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (p ParishCreate) Model() models.Parish {
	return models.Parish{
		Name:        p.Name,
		Church:      p.Church.ModelPtr(),
		Settlements: TextRefsToModel(p.Settlements),
		Since:       p.Since.Model(),
		Until:       p.Until.Model(),
		Notes:       TextRefsToModel(p.Notes),
	}
}

// ParishUpdate — тело PUT /api/parishes/{id} и аргументы тула parish_update:
// полная замена name/church/settlements/since/until/notes.
type ParishUpdate struct {
	Name        string    `json:"name"`
	Church      *TextRef  `json:"church,omitempty"`
	Settlements []TextRef `json:"settlements"`
	Since       *FactDate `json:"since,omitempty"`
	Until       *FactDate `json:"until,omitempty"`
	Notes       []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (p ParishUpdate) Model() models.Parish {
	return models.Parish{
		Name:        p.Name,
		Church:      p.Church.ModelPtr(),
		Settlements: TextRefsToModel(p.Settlements),
		Since:       p.Since.Model(),
		Until:       p.Until.Model(),
		Notes:       TextRefsToModel(p.Notes),
	}
}
