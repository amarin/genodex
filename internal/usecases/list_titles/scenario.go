package list_titles

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список словарных записей фамилий».
type Scenario struct {
	titles TitleRepo
}

// New создаёт сценарий.
func New(titles TitleRepo) *Scenario {
	return &Scenario{titles: titles}
}

// ListTitles возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListTitles(ctx context.Context, access models.Access, page models.Page) ([]models.Title, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeTitle, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeTitle, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.titles.ListTitles(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Title, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
