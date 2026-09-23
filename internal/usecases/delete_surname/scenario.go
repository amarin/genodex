package delete_surname

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление словарной записи фамилии».
type Scenario struct {
	surnames SurnameRepo
}

// New создаёт сценарий.
func New(surnames SurnameRepo) *Scenario {
	return &Scenario{surnames: surnames}
}

// DeleteSurname удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteSurname(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.surnames.DeleteSurname(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeSurname)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeSurname,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
