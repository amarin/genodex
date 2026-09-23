package delete_source

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление записи источника».
type Scenario struct {
	sources SourceRepo
}

// New создаёт сценарий.
func New(sources SourceRepo) *Scenario {
	return &Scenario{sources: sources}
}

// DeleteSource удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteSource(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.sources.DeleteSource(ctx, id)
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
