package list_sources

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список записей источников».
type Scenario struct {
	sources SourceRepo
}

// New создаёт сценарий.
func New(sources SourceRepo) *Scenario {
	return &Scenario{sources: sources}
}

// ListSources возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListSources(ctx context.Context, access models.Access, page models.Page) ([]models.Source, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeSource, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeSource, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.sources.ListSources(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Source, 0, len(list))
	for _, a := range list {
		out = append(out, *a)
	}

	return out, nil
}
