package list_estates

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список словарных записей фамилий».
type Scenario struct {
	estates EstateRepo
}

// New создаёт сценарий.
func New(estates EstateRepo) *Scenario {
	return &Scenario{estates: estates}
}

// ListEstates возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListEstates(ctx context.Context, access models.Access, page models.Page) ([]models.Estate, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeEstate, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeEstate, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.estates.ListEstates(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Estate, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
