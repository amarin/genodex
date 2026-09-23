package delete_title

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление словарной записи фамилии».
type Scenario struct {
	titles TitleRepo
}

// New создаёт сценарий.
func New(titles TitleRepo) *Scenario {
	return &Scenario{titles: titles}
}

// DeleteTitle удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteTitle(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.titles.DeleteTitle(ctx, id)
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
