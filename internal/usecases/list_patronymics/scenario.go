package list_patronymics

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список словарных записей отчеств».
type Scenario struct {
	patronymics PatronymicRepo
}

// New создаёт сценарий.
func New(patronymics PatronymicRepo) *Scenario {
	return &Scenario{patronymics: patronymics}
}

// ListPatronymics возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListPatronymics(ctx context.Context, access models.Access, page models.Page) ([]models.Patronymic, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypePatronymic, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypePatronymic, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.patronymics.ListPatronymics(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Patronymic, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
