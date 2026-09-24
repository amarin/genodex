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
// List/Search. Тот же принцип распространяется на цитаты, на которые
// ссылается хранилище через Sources: если хотя бы один
// Sources[i].CitationID указывает на приватную цитату, запись целиком
// прячется как отсутствующее, даже если Private == false у самой записи —
// иначе публичная запись выдаёт факт существования и id приватной цитаты.
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

	if access != models.AccessFull {
		hidden, err := repositoryReferencesPrivateCitation(ctx, s.repositories, r)
		if err != nil {
			return models.Repository{}, err
		}

		if hidden {
			return models.Repository{}, models.ErrNotFound
		}
	}

	return *r, nil
}

// repositoryReferencesPrivateCitation сообщает, ссылается ли запись (через
// Sources[i].CitationID) хотя бы на одну приватную цитату.
func repositoryReferencesPrivateCitation(ctx context.Context, repo RepositoryRepo, rec *models.Repository) (bool, error) {
	for _, sl := range rec.Sources {
		c, err := repo.GetCitation(ctx, sl.CitationID)
		if err != nil {
			return false, err
		}

		if c.Private {
			return true, nil
		}
	}

	return false, nil
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
