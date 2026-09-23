package transport

import "github.com/amarin/genodex/internal/models"

// ArchiveDocument — контракт документа внутри единицы учёта (GET
// /api/archive-documents, MCP-тул archive_document_list). UnitID —
// обязательная строгая ссылка на ArchiveNode (единицу учёта, к которой
// относится документ).
type ArchiveDocument struct {
	ID          models.ID    `json:"id"`
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

// ArchiveDocumentFromModel конвертирует запись в контракт.
func ArchiveDocumentFromModel(d models.ArchiveDocument) ArchiveDocument {
	return ArchiveDocument{
		ID:          d.ID,
		UnitID:      d.UnitID,
		Title:       d.Title,
		Kind:        d.Kind,
		Since:       FactDateFromModel(d.Since),
		Until:       FactDateFromModel(d.Until),
		Parish:      TextRefFromModelPtr(d.Parish),
		Settlements: TextRefsFromModel(d.Settlements),
		Notes:       TextRefsFromModel(d.Notes),
		Sources:     SourceLinksFromModel(d.Sources),
		Private:     d.Private,
	}
}

// ArchiveDocumentsFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func ArchiveDocumentsFromModels(ds []models.ArchiveDocument) []ArchiveDocument {
	out := make([]ArchiveDocument, 0, len(ds))
	for _, d := range ds {
		out = append(out, ArchiveDocumentFromModel(d))
	}

	return out
}
