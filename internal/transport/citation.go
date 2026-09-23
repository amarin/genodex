package transport

import "github.com/amarin/genodex/internal/models"

// Citation — контракт цитаты из источника (GET /api/citations, MCP-тул
// citation_list). Anchor — полиморфная привязка «где именно» (см.
// transport.Anchor), опциональна.
type Citation struct {
	ID       models.ID `json:"id"`
	SourceID string    `json:"source_id"`
	Anchor   *Anchor   `json:"anchor,omitempty"`
	Text     string    `json:"text,omitempty"`
	Note     string    `json:"note,omitempty"`
	Private  bool      `json:"private"`
}

// CitationFromModel конвертирует запись в контракт.
func CitationFromModel(c models.Citation) Citation {
	return Citation{
		ID:       c.ID,
		SourceID: string(c.SourceID),
		Anchor:   AnchorFromModel(c.Anchor),
		Text:     c.Text,
		Note:     c.Note,
		Private:  c.Private,
	}
}

// CitationsFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func CitationsFromModels(cs []models.Citation) []Citation {
	out := make([]Citation, 0, len(cs))
	for _, c := range cs {
		out = append(out, CitationFromModel(c))
	}

	return out
}
