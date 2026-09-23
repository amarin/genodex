package get_title

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись фамилии по идентификатору».
type Scenario struct {
	titles TitleRepo
}

// New создаёт сценарий.
func New(titles TitleRepo) *Scenario {
	return &Scenario{titles: titles}
}

// GetTitle возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetTitle(ctx context.Context, id models.ID) (models.Title, error) {
	if err := validateID(id); err != nil {
		return models.Title{}, err
	}

	sn, err := s.titles.GetTitle(ctx, id)
	if err != nil {
		return models.Title{}, err
	}

	return *sn, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeTitle)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeTitle,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
