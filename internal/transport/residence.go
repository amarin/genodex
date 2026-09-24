package transport

import "github.com/amarin/genodex/internal/models"

// Residence — контракт проживания персоны в месте (GET /api/residences,
// MCP-тул residence_list). Note — единственная строка (не []TextRef, в
// отличие от Notes у большинства сущностей) — отражает models.Residence.Note
// как есть.
type Residence struct {
	ID       models.ID    `json:"id"`
	PersonID models.ID    `json:"person_id"`
	PlaceID  models.ID    `json:"place_id"`
	Since    *FactDate    `json:"since,omitempty"`
	Until    *FactDate    `json:"until,omitempty"`
	Sources  []SourceLink `json:"sources"`
	Note     string       `json:"note,omitempty"`
	Private  bool         `json:"private"`
}

// ResidenceFromModel конвертирует запись в контракт.
func ResidenceFromModel(r models.Residence) Residence {
	return Residence{
		ID:       r.ID,
		PersonID: r.PersonID,
		PlaceID:  r.PlaceID,
		Since:    FactDateFromModel(r.Since),
		Until:    FactDateFromModel(r.Until),
		Sources:  SourceLinksFromModel(r.Sources),
		Note:     r.Note,
		Private:  r.Private,
	}
}

// ResidencesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func ResidencesFromModels(rs []models.Residence) []Residence {
	out := make([]Residence, 0, len(rs))
	for _, r := range rs {
		out = append(out, ResidenceFromModel(r))
	}

	return out
}
