package get_residence

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «проживание по идентификатору».
type Scenario struct {
	residences ResidenceRepo
}

// New создаёт сценарий.
func New(residences ResidenceRepo) *Scenario {
	return &Scenario{residences: residences}
}

// GetResidence возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound.
func (s *Scenario) GetResidence(ctx context.Context, access models.Access, id models.ID) (models.Residence, error) {
	if err := validateID(id); err != nil {
		return models.Residence{}, err
	}

	r, err := s.residences.GetResidence(ctx, id)
	if err != nil {
		return models.Residence{}, err
	}

	if r.Private && access != models.AccessFull {
		return models.Residence{}, models.ErrNotFound
	}

	return *r, nil
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
