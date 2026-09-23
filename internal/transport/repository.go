package transport

import "github.com/amarin/genodex/internal/models"

// Repository — контракт хранилища-контейнера источников (GET /api/repositories,
// MCP-тул repository_list). Sources — read-only в v1 (см. internal/transport/source_link.go).
type Repository struct {
	ID      models.ID    `json:"id"`
	Name    string       `json:"name"`
	Type    string       `json:"type"`
	Address string       `json:"address,omitempty"`
	URLs    []TextRef    `json:"urls"`
	Notes   []TextRef    `json:"notes"`
	Sources []SourceLink `json:"sources"`
	Private bool         `json:"private"`
}

// RepositoryFromModel конвертирует запись в контракт.
func RepositoryFromModel(r models.Repository) Repository {
	return Repository{
		ID:      r.ID,
		Name:    r.Name,
		Type:    string(r.Type),
		Address: r.Address,
		URLs:    TextRefsFromModel(r.URLs),
		Notes:   TextRefsFromModel(r.Notes),
		Sources: SourceLinksFromModel(r.Sources),
		Private: r.Private,
	}
}

// RepositoriesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func RepositoriesFromModels(rs []models.Repository) []Repository {
	out := make([]Repository, 0, len(rs))
	for _, r := range rs {
		out = append(out, RepositoryFromModel(r))
	}

	return out
}
