package models

// SourceLink — доказательство: связь «утверждение → источник».
// Применяется к любому утверждению любой сущности.
type SourceLink struct {
	// SourceID — id источника (строгая ссылка).
	SourceID string `json:"source_id"`
	// TargetType — тип сущности-утверждения.
	TargetType string `json:"target_type"`
	// TargetID — id сущности-утверждения.
	TargetID string `json:"target_id"`
	// Reliability — частная достоверность именно этого утверждения
	// по этому источнику.
	Reliability string `json:"reliability,omitempty"`
	// Role — роль утверждения.
	Role string `json:"role,omitempty"`
	// Note — примечание.
	Note string `json:"note,omitempty"`
}
