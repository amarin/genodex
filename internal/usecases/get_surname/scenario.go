package get_surname

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись фамилии по идентификатору».
type Scenario struct {
	surnames SurnameRepo
}

// New создаёт сценарий.
func New(surnames SurnameRepo) *Scenario {
	return &Scenario{surnames: surnames}
}

// GetSurname возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetSurname(ctx context.Context, id models.ID) (models.Surname, error) {
	if err := validateID(id); err != nil {
		return models.Surname{}, err
	}

	sn, err := s.surnames.GetSurname(ctx, id)
	if err != nil {
		return models.Surname{}, err
	}

	return *sn, nil
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
