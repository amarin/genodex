package transport

import "github.com/amarin/genodex/internal/models"

// Source — контракт источника доказательства (GET /api/sources, MCP-тул
// source_list). Date — структурированная дата (см. transport.FactDate).
// RepositoryID — мягкая ссылка на хранилище (просто id, необязательна).
type Source struct {
	ID           models.ID `json:"id"`
	Kind         string    `json:"kind"`
	Title        string    `json:"title"`
	Author       string    `json:"author,omitempty"`
	Date         *FactDate `json:"date,omitempty"`
	Reliability  string    `json:"reliability"`
	RepositoryID string    `json:"repository_id,omitempty"`
	Notes        []TextRef `json:"notes"`
	Private      bool      `json:"private"`
}

// SourceFromModel конвертирует запись в контракт.
func SourceFromModel(s models.Source) Source {
	return Source{
		ID:           s.ID,
		Kind:         string(s.Kind),
		Title:        s.Title,
		Author:       s.Author,
		Date:         FactDateFromModel(s.Date),
		Reliability:  string(s.Reliability),
		RepositoryID: string(s.RepositoryID),
		Notes:        TextRefsFromModel(s.Notes),
		Private:      s.Private,
	}
}

// SourcesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func SourcesFromModels(ss []models.Source) []Source {
	out := make([]Source, 0, len(ss))
	for _, s := range ss {
		out = append(out, SourceFromModel(s))
	}

	return out
}
