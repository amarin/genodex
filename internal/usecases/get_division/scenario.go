package get_division

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «единица административного деления по идентификатору».
type Scenario struct {
	divisions DivisionRepo
}

// New создаёт сценарий.
func New(divisions DivisionRepo) *Scenario {
	return &Scenario{divisions: divisions}
}

// GetDivision возвращает единицу деления по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой единицы — models.ErrNotFound.
func (s *Scenario) GetDivision(ctx context.Context, id models.ID) (models.AdministrativeDivision, error) {
	if err := validateID(id); err != nil {
		return models.AdministrativeDivision{}, err
	}

	d, err := s.divisions.GetAdministrativeDivision(ctx, id)
	if err != nil {
		return models.AdministrativeDivision{}, err
	}

	return *d, nil
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
