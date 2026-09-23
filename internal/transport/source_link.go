package transport

import "github.com/amarin/genodex/internal/models"

// SourceLink — контракт доказательства «утверждение → цитата»
// (models.SourceLink). Редактируемый с подпроекта 5 (Citation теперь имеет
// CRUD): Create/Update DTO сущностей-владельцев (Repository/Church/Parish/
// Archive/Note/AdministrativeDivision) несут это поле. TargetType/TargetID
// клиент не отправляет и они игнорируются при сохранении — владелец
// восстанавливается сценарием/хранилищем из контекста вызова (см.
// sqlstore.replaceSourceLinks/loadSourceLinks), поэтому Model() их не
// заполняет.
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

// Model конвертирует контракт обратно в модель. TargetType/TargetID не
// заполняются — владелец подставляет их сам (см. тип выше).
func (s SourceLink) Model() models.SourceLink {
	return models.SourceLink{
		CitationID:  models.ID(s.CitationID),
		Reliability: models.Reliability(s.Reliability),
		Role:        s.Role,
		Note:        s.Note,
	}
}

// SourceLinksToModel конвертирует список контрактов в модели; пустой вход
// даёт пустой срез, а не nil.
func SourceLinksToModel(ss []SourceLink) []models.SourceLink {
	out := make([]models.SourceLink, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.Model())
	}

	return out
}
