package transport

import "github.com/amarin/genodex/internal/models"

// ArchiveCreate — тело POST /api/archives и аргументы тула archive_create.
// Идентификатор генерирует сценарий. RepositoryID — просто id (пустая строка —
// без хранилища); сценарий проверяет существование при непустом значении.
type ArchiveCreate struct {
	Name         string       `json:"name"`
	System       *TextRef     `json:"system,omitempty"`
	RepositoryID string       `json:"repository_id,omitempty"`
	Notes        []TextRef    `json:"notes"`
	Sources      []SourceLink `json:"sources"`
	Private      bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (a ArchiveCreate) Model() models.Archive {
	return models.Archive{
		Name:         a.Name,
		System:       a.System.ModelPtr(),
		RepositoryID: models.ID(a.RepositoryID),
		Notes:        TextRefsToModel(a.Notes),
		Sources:      SourceLinksToModel(a.Sources),
		Private:      a.Private,
	}
}

// ArchiveUpdate — тело PUT /api/archives/{id} и аргументы тула archive_update:
// полная замена name/system/repository_id/notes/sources/private.
type ArchiveUpdate struct {
	Name         string       `json:"name"`
	System       *TextRef     `json:"system,omitempty"`
	RepositoryID string       `json:"repository_id,omitempty"`
	Notes        []TextRef    `json:"notes"`
	Sources      []SourceLink `json:"sources"`
	Private      bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (a ArchiveUpdate) Model() models.Archive {
	return models.Archive{
		Name:         a.Name,
		System:       a.System.ModelPtr(),
		RepositoryID: models.ID(a.RepositoryID),
		Notes:        TextRefsToModel(a.Notes),
		Sources:      SourceLinksToModel(a.Sources),
		Private:      a.Private,
	}
}
