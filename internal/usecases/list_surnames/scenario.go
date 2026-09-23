package list_surnames

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список словарных записей фамилий».
type Scenario struct {
	surnames SurnameRepo
}

// New создаёт сценарий.
func New(surnames SurnameRepo) *Scenario {
	return &Scenario{surnames: surnames}
}

// ListSurnames возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListSurnames(ctx context.Context, access models.Access, page models.Page) ([]models.Surname, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeSurname, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeSurname, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.surnames.ListSurnames(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Surname, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
