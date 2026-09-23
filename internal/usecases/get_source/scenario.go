package get_source

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «запись источника по идентификатору».
type Scenario struct {
	sources SourceRepo
}

// New создаёт сценарий.
func New(sources SourceRepo) *Scenario {
	return &Scenario{sources: sources}
}

// GetSource возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search.
func (s *Scenario) GetSource(ctx context.Context, access models.Access, id models.ID) (models.Source, error) {
	if err := validateID(id); err != nil {
		return models.Source{}, err
	}

	src, err := s.sources.GetSource(ctx, id)
	if err != nil {
		return models.Source{}, err
	}

	if src.Private && access != models.AccessFull {
		return models.Source{}, models.ErrNotFound
	}

	return *src, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeSource)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeSource,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
