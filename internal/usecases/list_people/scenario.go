package list_people

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список персон».
type Scenario struct {
	people PersonRepo
}

// New создаёт сценарий.
func New(people PersonRepo) *Scenario {
	return &Scenario{people: people}
}

// ListPeople возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListPeople(ctx context.Context, access models.Access, page models.Page) ([]models.Person, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypePerson, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypePerson, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.people.ListPeople(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Person, 0, len(list))
	for _, p := range list {
		out = append(out, *p)
	}

	return out, nil
}
