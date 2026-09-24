package transport

import "github.com/amarin/genodex/internal/models"

// PlaceRef — контракт «указания на место» (models.PlaceRef): текст или
// ссылка, чей тип ограничен местом (административное деление, церковь или
// приход). Структурно идентичен TextRef ({text, ref, type}), но это
// отдельный тип модельного слоя (models.PlaceRef != models.TextRef), поэтому
// заводится собственный транспортный тип и собственные конвертеры, а не
// переиспользуется transport.TextRef. Единственное текущее поле такой формы
// в программе — Event.Place (docs/data-model/entity-write.md §3.8).
type PlaceRef struct {
	Text string `json:"text"`
	Ref  string `json:"ref,omitempty"`
	Type string `json:"type,omitempty"`
}

// PlaceRefFromModel конвертирует указание на место в контракт; nil — не задано.
func PlaceRefFromModel(p *models.PlaceRef) *PlaceRef {
	if p == nil {
		return nil
	}

	return &PlaceRef{Text: p.Text, Ref: string(p.Ref), Type: string(p.Type)}
}

// Model конвертирует контракт обратно в модель; nil — не задано.
func (p *PlaceRef) Model() *models.PlaceRef {
	if p == nil {
		return nil
	}

	return &models.PlaceRef{Text: p.Text, Ref: models.ID(p.Ref), Type: models.Type(p.Type)}
}
