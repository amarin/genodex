package delete_division

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление единицы административного деления».
type Scenario struct {
	divisions DivisionRepo
}

// New создаёт сценарий.
func New(divisions DivisionRepo) *Scenario {
	return &Scenario{divisions: divisions}
}

// DeleteDivision удаляет единицу деления. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// единицы — models.ErrNotFound; на единицу ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteDivision(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.divisions.DeleteAdministrativeDivision(ctx, id)
}

// validateID проверяет формат идентификатора деления; ошибка —
// *models.ValidationError по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeAdministrativeDivision)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeAdministrativeDivision,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
