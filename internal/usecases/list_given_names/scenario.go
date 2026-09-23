package list_given_names

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список словарных записей имён».
type Scenario struct {
	givenNames GivenNameRepo
}

// New создаёт сценарий.
func New(givenNames GivenNameRepo) *Scenario {
	return &Scenario{givenNames: givenNames}
}

// ListGivenNames возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListGivenNames(ctx context.Context, access models.Access, page models.Page) ([]models.GivenName, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeGivenName, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeGivenName, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.givenNames.ListGivenNames(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.GivenName, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
