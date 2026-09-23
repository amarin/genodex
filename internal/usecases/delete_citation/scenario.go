package delete_citation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление цитаты».
type Scenario struct {
	citations CitationRepo
}

// New создаёт сценарий.
func New(citations CitationRepo) *Scenario {
	return &Scenario{citations: citations}
}

// DeleteCitation удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие (SourceLink.CitationID
// у любой сущности с доказательствами) — *models.InUseError со списком
// ссылающихся.
func (s *Scenario) DeleteCitation(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.citations.DeleteCitation(ctx, id)
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
