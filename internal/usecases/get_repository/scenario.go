package get_repository

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «запись хранилища по идентификатору».
type Scenario struct {
	repositories RepositoryRepo
}

// New создаёт сценарий.
func New(repositories RepositoryRepo) *Scenario {
	return &Scenario{repositories: repositories}
}

// GetRepository возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search.
func (s *Scenario) GetRepository(ctx context.Context, access models.Access, id models.ID) (models.Repository, error) {
	if err := validateID(id); err != nil {
		return models.Repository{}, err
	}

	r, err := s.repositories.GetRepository(ctx, id)
	if err != nil {
		return models.Repository{}, err
	}

	if r.Private && access != models.AccessFull {
		return models.Repository{}, models.ErrNotFound
	}

	return *r, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeRepository)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeRepository,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
