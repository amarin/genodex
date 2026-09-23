package transport

import "github.com/amarin/genodex/internal/models"

// ArchiveDocumentCreate — тело POST /api/archive-documents и аргументы тула
// archive_document_create. Идентификатор генерирует сценарий.
type ArchiveDocumentCreate struct {
	UnitID      models.ID    `json:"unit_id"`
	Title       string       `json:"title"`
	Kind        string       `json:"kind"`
	Since       *FactDate    `json:"since,omitempty"`
	Until       *FactDate    `json:"until,omitempty"`
	Parish      *TextRef     `json:"parish,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
	Private     bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (d ArchiveDocumentCreate) Model() models.ArchiveDocument {
	return models.ArchiveDocument{
		UnitID:      d.UnitID,
		Title:       d.Title,
		Kind:        d.Kind,
		Since:       d.Since.Model(),
		Until:       d.Until.Model(),
		Parish:      d.Parish.ModelPtr(),
		Settlements: TextRefsToModel(d.Settlements),
		Notes:       TextRefsToModel(d.Notes),
		Sources:     SourceLinksToModel(d.Sources),
		Private:     d.Private,
	}
}

// ArchiveDocumentUpdate — тело PUT /api/archive-documents/{id} и аргументы
// тула archive_document_update: полная замена всех полей ниже id.
type ArchiveDocumentUpdate struct {
	UnitID      models.ID    `json:"unit_id"`
	Title       string       `json:"title"`
	Kind        string       `json:"kind"`
	Since       *FactDate    `json:"since,omitempty"`
	Until       *FactDate    `json:"until,omitempty"`
	Parish      *TextRef     `json:"parish,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
	Private     bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (d ArchiveDocumentUpdate) Model() models.ArchiveDocument {
	return models.ArchiveDocument{
		UnitID:      d.UnitID,
		Title:       d.Title,
		Kind:        d.Kind,
		Since:       d.Since.Model(),
		Until:       d.Until.Model(),
		Parish:      d.Parish.ModelPtr(),
		Settlements: TextRefsToModel(d.Settlements),
		Notes:       TextRefsToModel(d.Notes),
		Sources:     SourceLinksToModel(d.Sources),
		Private:     d.Private,
	}
}
