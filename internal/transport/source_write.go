package transport

import "github.com/amarin/genodex/internal/models"

// SourceCreate — тело POST /api/sources и аргументы тула source_create.
// Идентификатор генерирует сценарий. RepositoryID — просто id (пустая
// строка — без хранилища); сценарий проверяет существование при непустом
// значении, по образцу create_archive.
type SourceCreate struct {
	Kind         string    `json:"kind"`
	Title        string    `json:"title"`
	Author       string    `json:"author,omitempty"`
	Date         *FactDate `json:"date,omitempty"`
	Reliability  string    `json:"reliability"`
	RepositoryID string    `json:"repository_id,omitempty"`
	Notes        []TextRef `json:"notes"`
	Private      bool      `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (s SourceCreate) Model() models.Source {
	return models.Source{
		Kind:         models.SourceKind(s.Kind),
		Title:        s.Title,
		Author:       s.Author,
		Date:         s.Date.Model(),
		Reliability:  models.Reliability(s.Reliability),
		RepositoryID: models.ID(s.RepositoryID),
		Notes:        TextRefsToModel(s.Notes),
		Private:      s.Private,
	}
}

// SourceUpdate — тело PUT /api/sources/{id} и аргументы тула source_update:
// полная замена kind/title/author/date/reliability/repository_id/notes/private.
type SourceUpdate struct {
	Kind         string    `json:"kind"`
	Title        string    `json:"title"`
	Author       string    `json:"author,omitempty"`
	Date         *FactDate `json:"date,omitempty"`
	Reliability  string    `json:"reliability"`
	RepositoryID string    `json:"repository_id,omitempty"`
	Notes        []TextRef `json:"notes"`
	Private      bool      `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (s SourceUpdate) Model() models.Source {
	return models.Source{
		Kind:         models.SourceKind(s.Kind),
		Title:        s.Title,
		Author:       s.Author,
		Date:         s.Date.Model(),
		Reliability:  models.Reliability(s.Reliability),
		RepositoryID: models.ID(s.RepositoryID),
		Notes:        TextRefsToModel(s.Notes),
		Private:      s.Private,
	}
}
