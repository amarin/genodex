package get_patronymic

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись отчества по идентификатору».
type Scenario struct {
	patronymics PatronymicRepo
}

// New создаёт сценарий.
func New(patronymics PatronymicRepo) *Scenario {
	return &Scenario{patronymics: patronymics}
}

// GetPatronymic возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetPatronymic(ctx context.Context, id models.ID) (models.Patronymic, error) {
	if err := validateID(id); err != nil {
		return models.Patronymic{}, err
	}

	sn, err := s.patronymics.GetPatronymic(ctx, id)
	if err != nil {
		return models.Patronymic{}, err
	}

	return *sn, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypePatronymic)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypePatronymic,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
