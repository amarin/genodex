package transport

import "github.com/amarin/genodex/internal/models"

// CitationCreate — тело POST /api/citations и аргументы тула
// citation_create. Идентификатор генерирует сценарий. SourceID — строгая
// ссылка на источник (сценарий проверяет существование). Anchor —
// необязательная полиморфная привязка (см. transport.Anchor).
type CitationCreate struct {
	SourceID string  `json:"source_id"`
	Anchor   *Anchor `json:"anchor,omitempty"`
	Text     string  `json:"text,omitempty"`
	Note     string  `json:"note,omitempty"`
	Private  bool    `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (c CitationCreate) Model() models.Citation {
	return models.Citation{
		SourceID: models.ID(c.SourceID),
		Anchor:   c.Anchor.Model(),
		Text:     c.Text,
		Note:     c.Note,
		Private:  c.Private,
	}
}

// CitationUpdate — тело PUT /api/citations/{id} и аргументы тула
// citation_update: полная замена source_id/anchor/text/note/private.
type CitationUpdate struct {
	SourceID string  `json:"source_id"`
	Anchor   *Anchor `json:"anchor,omitempty"`
	Text     string  `json:"text,omitempty"`
	Note     string  `json:"note,omitempty"`
	Private  bool    `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (c CitationUpdate) Model() models.Citation {
	return models.Citation{
		SourceID: models.ID(c.SourceID),
		Anchor:   c.Anchor.Model(),
		Text:     c.Text,
		Note:     c.Note,
		Private:  c.Private,
	}
}
