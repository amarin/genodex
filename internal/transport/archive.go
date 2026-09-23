package transport

import "github.com/amarin/genodex/internal/models"

// Archive — контракт архива (GET /api/archives, MCP-тул archive_list). System —
// система иерархии архива, задаётся только именем (text), ссылка на сущность
// не допускается (models.Archive.Validate). RepositoryID — необязательная
// строгая ссылка на хранилище (просто id, не TextRef — в отличие от
// Church.Parish/Parish.Church): пустая строка — не задана.
type Archive struct {
	ID           models.ID    `json:"id"`
	Name         string       `json:"name"`
	System       *TextRef     `json:"system,omitempty"`
	RepositoryID string       `json:"repository_id,omitempty"`
	Notes        []TextRef    `json:"notes"`
	Sources      []SourceLink `json:"sources"`
	Private      bool         `json:"private"`
}

// ArchiveFromModel конвертирует запись в контракт.
func ArchiveFromModel(a models.Archive) Archive {
	return Archive{
		ID:           a.ID,
		Name:         a.Name,
		System:       TextRefFromModelPtr(a.System),
		RepositoryID: string(a.RepositoryID),
		Notes:        TextRefsFromModel(a.Notes),
		Sources:      SourceLinksFromModel(a.Sources),
		Private:      a.Private,
	}
}

// ArchivesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func ArchivesFromModels(as []models.Archive) []Archive {
	out := make([]Archive, 0, len(as))
	for _, a := range as {
		out = append(out, ArchiveFromModel(a))
	}

	return out
}
