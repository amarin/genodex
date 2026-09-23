package transport

import "github.com/amarin/genodex/internal/models"

// SourceLink — контракт доказательства «утверждение → цитата» (models.SourceLink),
// read-only в v1: Citation ещё не имеет CRUD (docs/data-model/entity-write.md §2,
// подпроект 5), поэтому Create/Update DTO это поле не несут — только чтение
// уже существующих записей (заведены напрямую в БД/через MCP руками, если
// вообще есть). Появляется впервые в этом проходе (первые сущности с полем
// Sources), переиспользуется везде, где встречается []models.SourceLink.
type SourceLink struct {
	CitationID  string `json:"citation_id"`
	TargetType  string `json:"target_type,omitempty"`
	TargetID    string `json:"target_id,omitempty"`
	Reliability string `json:"reliability,omitempty"`
	Role        string `json:"role,omitempty"`
	Note        string `json:"note,omitempty"`
}

// SourceLinkFromModel конвертирует запись в контракт.
func SourceLinkFromModel(s models.SourceLink) SourceLink {
	return SourceLink{
		CitationID:  string(s.CitationID),
		TargetType:  string(s.TargetType),
		TargetID:    string(s.TargetID),
		Reliability: string(s.Reliability),
		Role:        s.Role,
		Note:        s.Note,
	}
}

// SourceLinksFromModel конвертирует список; пустой вход даёт пустой срез, а не nil.
func SourceLinksFromModel(ss []models.SourceLink) []SourceLink {
	out := make([]SourceLink, 0, len(ss))
	for _, s := range ss {
		out = append(out, SourceLinkFromModel(s))
	}

	return out
}
