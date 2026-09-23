package transport

import "github.com/amarin/genodex/internal/models"

// RepositoryCreate — тело POST /api/repositories и аргументы тула
// repository_create. Идентификатор генерирует сценарий. Sources не входит —
// read-only в v1 (см. source_link.go).
type RepositoryCreate struct {
	Name    string    `json:"name"`
	Type    string    `json:"type"`
	Address string    `json:"address,omitempty"`
	URLs    []TextRef `json:"urls"`
	Notes   []TextRef `json:"notes"`
	Private bool      `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (r RepositoryCreate) Model() models.Repository {
	return models.Repository{
		Name:    r.Name,
		Type:    models.RepositoryType(r.Type),
		Address: r.Address,
		URLs:    TextRefsToModel(r.URLs),
		Notes:   TextRefsToModel(r.Notes),
		Private: r.Private,
	}
}

// RepositoryUpdate — тело PUT /api/repositories/{id} и аргументы тула
// repository_update: полная замена name/type/address/urls/notes/private.
// Sources не входит — read-only в v1, fetch-then-merge сохраняет текущее
// значение (internal/httpapi/repository_write.go).
type RepositoryUpdate struct {
	Name    string    `json:"name"`
	Type    string    `json:"type"`
	Address string    `json:"address,omitempty"`
	URLs    []TextRef `json:"urls"`
	Notes   []TextRef `json:"notes"`
	Private bool      `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (r RepositoryUpdate) Model() models.Repository {
	return models.Repository{
		Name:    r.Name,
		Type:    models.RepositoryType(r.Type),
		Address: r.Address,
		URLs:    TextRefsToModel(r.URLs),
		Notes:   TextRefsToModel(r.Notes),
		Private: r.Private,
	}
}
