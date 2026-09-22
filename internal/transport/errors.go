package transport

import "github.com/amarin/genodex/internal/models"

// Ref — ссылка на сущность в теле ответа 409: тип и идентификатор.
type Ref struct {
	Type models.Type `json:"type"`
	ID   models.ID   `json:"id"`
}

// InUseErrorBody — тело ответа 409 Conflict: текст ошибки и первые
// models.MaxReferrers ссылающихся сущностей в стабильном порядке без повторов.
type InUseErrorBody struct {
	Error     string `json:"error"`
	Referrers []Ref  `json:"referrers"`
}

// InUseErrorBodyFromModel конвертирует ошибку занятости в тело ответа.
func InUseErrorBodyFromModel(e *models.InUseError) InUseErrorBody {
	refs := make([]Ref, 0, len(e.Referrers))
	for _, r := range e.Referrers {
		refs = append(refs, Ref{Type: r.Type, ID: r.ID})
	}
	return InUseErrorBody{Error: e.Error(), Referrers: refs}
}
