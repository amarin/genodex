package list_churches

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список записей церквей».
type Scenario struct {
	churches ChurchRepo
}

// New создаёт сценарий.
func New(churches ChurchRepo) *Scenario {
	return &Scenario{churches: churches}
}

// ListChurches возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListChurches(ctx context.Context, access models.Access, page models.Page) ([]models.Church, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeChurch, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeChurch, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.churches.ListChurches(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Church, 0, len(list))
	for _, c := range list {
		out = append(out, *c)
	}

	return out, nil
}
