package transport

import "github.com/amarin/genodex/internal/models"

// RepositoryCreate — тело POST /api/repositories и аргументы тула
// repository_create. Идентификатор генерирует сценарий.
type RepositoryCreate struct {
	Name    string       `json:"name"`
	Type    string       `json:"type"`
	Address string       `json:"address,omitempty"`
	URLs    []TextRef    `json:"urls"`
	Notes   []TextRef    `json:"notes"`
	Sources []SourceLink `json:"sources"`
	Private bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (r RepositoryCreate) Model() models.Repository {
	return models.Repository{
		Name:    r.Name,
		Type:    models.RepositoryType(r.Type),
		Address: r.Address,
		URLs:    TextRefsToModel(r.URLs),
		Notes:   TextRefsToModel(r.Notes),
		Sources: SourceLinksToModel(r.Sources),
		Private: r.Private,
	}
}

// RepositoryUpdate — тело PUT /api/repositories/{id} и аргументы тула
// repository_update: полная замена name/type/address/urls/notes/sources/private.
type RepositoryUpdate struct {
	Name    string       `json:"name"`
	Type    string       `json:"type"`
	Address string       `json:"address,omitempty"`
	URLs    []TextRef    `json:"urls"`
	Notes   []TextRef    `json:"notes"`
	Sources []SourceLink `json:"sources"`
	Private bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (r RepositoryUpdate) Model() models.Repository {
	return models.Repository{
		Name:    r.Name,
		Type:    models.RepositoryType(r.Type),
		Address: r.Address,
		URLs:    TextRefsToModel(r.URLs),
		Notes:   TextRefsToModel(r.Notes),
		Sources: SourceLinksToModel(r.Sources),
		Private: r.Private,
	}
}
