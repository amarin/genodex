package models

// Citation — цитата из источника: доказательственная привязка
// (якорь/движимое страницы/тайм-код/url) и выписка текстом.
// Цепочка доказательства: SourceLink.citation_id → Citation.source_id → Source
// (решение #21).
type Citation struct {
	ID       ID
	SourceID ID
	Anchor   Anchor
	Text     string
	Note     string
	Private  bool
}

// EntityType возвращает тип сущности.
func (c *Citation) EntityType() Type { return TypeCitation }
