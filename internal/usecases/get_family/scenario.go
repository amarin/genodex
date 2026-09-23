package get_family

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «запись рода по идентификатору».
type Scenario struct {
	families FamilyRepo
}

// New создаёт сценарий.
func New(families FamilyRepo) *Scenario {
	return &Scenario{families: families}
}

// GetFamily возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search.
func (s *Scenario) GetFamily(ctx context.Context, access models.Access, id models.ID) (models.Family, error) {
	if err := validateID(id); err != nil {
		return models.Family{}, err
	}

	f, err := s.families.GetFamily(ctx, id)
	if err != nil {
		return models.Family{}, err
	}

	if f.Private && access != models.AccessFull {
		return models.Family{}, models.ErrNotFound
	}

	return *f, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeFamily)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeFamily,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
