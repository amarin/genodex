package transport

import "github.com/amarin/genodex/internal/models"

// TextRef — контракт элемента списков вроде Surname.Variants: текст или
// ссылка на другую сущность. Ref/Type пустые — элемент чисто текстовый.
// v1 веб-формы редактируют только Text (docs/data-model/entity-write.md §4) —
// Ref/Type только читаются и переносятся как есть при сохранении остального
// списка.
type TextRef struct {
	Text string `json:"text"`
	Ref  string `json:"ref,omitempty"`
	Type string `json:"type,omitempty"`
}

// TextRefFromModel конвертирует элемент в контракт.
func TextRefFromModel(t models.TextRef) TextRef {
	return TextRef{Text: t.Text, Ref: string(t.Ref), Type: string(t.Type)}
}

// TextRefsFromModel конвертирует список; пустой вход даёт пустой срез, а не nil.
func TextRefsFromModel(ts []models.TextRef) []TextRef {
	out := make([]TextRef, 0, len(ts))
	for _, t := range ts {
		out = append(out, TextRefFromModel(t))
	}

	return out
}

// Model конвертирует контракт обратно в модель.
func (t TextRef) Model() models.TextRef {
	return models.TextRef{Text: t.Text, Ref: models.ID(t.Ref), Type: models.Type(t.Type)}
}

// TextRefsToModel конвертирует список контрактов в модели; пустой вход даёт
// пустой срез, а не nil.
func TextRefsToModel(ts []TextRef) []models.TextRef {
	out := make([]models.TextRef, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.Model())
	}

	return out
}

// TextRefFromModelPtr конвертирует необязательную ссылку (например,
// Church.Parish); nil — не задана.
func TextRefFromModelPtr(t *models.TextRef) *TextRef {
	if t == nil {
		return nil
	}

	v := TextRefFromModel(*t)

	return &v
}

// ModelPtr конвертирует контракт обратно в необязательную модель; nil — не задана.
func (t *TextRef) ModelPtr() *models.TextRef {
	if t == nil {
		return nil
	}

	v := t.Model()

	return &v
}
