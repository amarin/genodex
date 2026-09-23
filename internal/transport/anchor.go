package transport

import "github.com/amarin/genodex/internal/models"

// Anchor — контракт полиморфной привязки «где именно» (models.Anchor):
// первый полиморфный тип в программе. Плоское представление с
// дискриминатором Kind ("archive"/"file"/"url"), по образцу FactDate
// (все поля опциональны вместе, а не набор отдельных объектов-вариантов) —
// проще для JSON REST и MCP-объектного аргумента, чем вложенный union.
// nil — привязки нет (цитата может относиться к источнику целиком).
type Anchor struct {
	Kind         string `json:"kind,omitempty"`
	NodeID       string `json:"node_id,omitempty"`
	DocumentID   string `json:"document_id,omitempty"`
	Page         int    `json:"page,omitempty"`
	Rect         string `json:"rect,omitempty"`
	AttachmentID string `json:"attachment_id,omitempty"`
	Timecode     string `json:"timecode,omitempty"`
	URL          string `json:"url,omitempty"`
}

// AnchorFromModel конвертирует привязку в контракт; nil — привязки нет.
func AnchorFromModel(a models.Anchor) *Anchor {
	switch v := a.(type) {
	case nil:
		return nil
	case *models.ArchiveAnchor:
		if v == nil {
			return nil
		}

		return &Anchor{Kind: "archive", NodeID: string(v.NodeID), DocumentID: string(v.DocumentID), Page: v.Page, Rect: v.Rect}
	case *models.FileAnchor:
		if v == nil {
			return nil
		}

		return &Anchor{Kind: "file", AttachmentID: string(v.AttachmentID), Timecode: v.Timecode}
	case *models.URLAnchor:
		if v == nil {
			return nil
		}

		return &Anchor{Kind: "url", URL: v.URL}
	default:
		return nil
	}
}

// Model конвертирует контракт обратно в модель; nil (или неизвестный/пустой
// Kind) — привязки нет. Само поле Anchor у Citation необязательно —
// невалидный Kind просто даёт "без привязки", а не ошибку конвертации;
// содержательную проверку (например, обязательные поля внутри варианта)
// делает models.Citation.Validate().
func (a *Anchor) Model() models.Anchor {
	if a == nil {
		return nil
	}

	switch a.Kind {
	case "archive":
		return &models.ArchiveAnchor{NodeID: models.ID(a.NodeID), DocumentID: models.ID(a.DocumentID), Page: a.Page, Rect: a.Rect}
	case "file":
		return &models.FileAnchor{AttachmentID: models.ID(a.AttachmentID), Timecode: a.Timecode}
	case "url":
		return &models.URLAnchor{URL: a.URL}
	default:
		return nil
	}
}
