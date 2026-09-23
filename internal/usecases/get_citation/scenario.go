package get_citation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «цитата по идентификатору».
type Scenario struct {
	citations CitationRepo
}

// New создаёт сценарий.
func New(citations CitationRepo) *Scenario {
	return &Scenario{citations: citations}
}

// GetCitation возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search.
func (s *Scenario) GetCitation(ctx context.Context, access models.Access, id models.ID) (models.Citation, error) {
	if err := validateID(id); err != nil {
		return models.Citation{}, err
	}

	c, err := s.citations.GetCitation(ctx, id)
	if err != nil {
		return models.Citation{}, err
	}

	if c.Private && access != models.AccessFull {
		return models.Citation{}, models.ErrNotFound
	}

	return *c, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeCitation)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeCitation,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
