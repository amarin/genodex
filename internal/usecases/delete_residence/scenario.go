package delete_residence

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление проживания».
type Scenario struct {
	residences ResidenceRepo
}

// New создаёт сценарий.
func New(residences ResidenceRepo) *Scenario {
	return &Scenario{residences: residences}
}

// DeleteResidence удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteResidence(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.residences.DeleteResidence(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeResidence)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeResidence,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
