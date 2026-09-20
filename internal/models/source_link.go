package models

// SourceLink — доказательство: связь «утверждение → цитата» (решение #21).
// Применяется к любому утверждению любой сущности. Цепочка:
// SourceLink.CitationID → Citation.SourceID → Source.
type SourceLink struct {
	// CitationID — id цитаты (строгая ссылка).
	CitationID ID
	// TargetType — тип сущности-утверждения.
	TargetType Type
	// TargetID — id сущности-утверждения.
	TargetID ID
	// Reliability — частная достоверность именно этого утверждения по этой цитате.
	Reliability Reliability
	// Role — роль утверждения.
	Role string
	// Note — примечание.
	Note string
}
