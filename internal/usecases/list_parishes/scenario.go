package list_parishes

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список записей приходов».
type Scenario struct {
	parishes ParishRepo
}

// New создаёт сценарий.
func New(parishes ParishRepo) *Scenario {
	return &Scenario{parishes: parishes}
}

// ListParishes возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListParishes(ctx context.Context, access models.Access, page models.Page) ([]models.Parish, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeParish, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeParish, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.parishes.ListParishes(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Parish, 0, len(list))
	for _, p := range list {
		out = append(out, *p)
	}

	return out, nil
}
