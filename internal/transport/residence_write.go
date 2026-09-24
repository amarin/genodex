package transport

import "github.com/amarin/genodex/internal/models"

// ResidenceCreate — тело POST /api/residences и аргументы тула
// residence_create. Идентификатор генерирует сценарий.
type ResidenceCreate struct {
	PersonID string       `json:"person_id"`
	PlaceID  string       `json:"place_id"`
	Since    *FactDate    `json:"since,omitempty"`
	Until    *FactDate    `json:"until,omitempty"`
	Sources  []SourceLink `json:"sources"`
	Note     string       `json:"note,omitempty"`
	Private  bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (r ResidenceCreate) Model() models.Residence {
	return models.Residence{
		PersonID: models.ID(r.PersonID),
		PlaceID:  models.ID(r.PlaceID),
		Since:    r.Since.Model(),
		Until:    r.Until.Model(),
		Sources:  SourceLinksToModel(r.Sources),
		Note:     r.Note,
		Private:  r.Private,
	}
}

// ResidenceUpdate — тело PUT /api/residences/{id} и аргументы тула
// residence_update: полная замена person_id/place_id/since/until/sources/
// note/private.
type ResidenceUpdate struct {
	PersonID string       `json:"person_id"`
	PlaceID  string       `json:"place_id"`
	Since    *FactDate    `json:"since,omitempty"`
	Until    *FactDate    `json:"until,omitempty"`
	Sources  []SourceLink `json:"sources"`
	Note     string       `json:"note,omitempty"`
	Private  bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (r ResidenceUpdate) Model() models.Residence {
	return models.Residence{
		PersonID: models.ID(r.PersonID),
		PlaceID:  models.ID(r.PlaceID),
		Since:    r.Since.Model(),
		Until:    r.Until.Model(),
		Sources:  SourceLinksToModel(r.Sources),
		Note:     r.Note,
		Private:  r.Private,
	}
}
