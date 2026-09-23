package list_repositories

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список словарных записей фамилий».
type Scenario struct {
	repositories RepositoryRepo
}

// New создаёт сценарий.
func New(repositories RepositoryRepo) *Scenario {
	return &Scenario{repositories: repositories}
}

// ListRepositories возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]models.Repository, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeRepository, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeRepository, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.repositories.ListRepositories(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Repository, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
