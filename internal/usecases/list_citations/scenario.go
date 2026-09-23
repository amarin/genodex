package list_citations

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список цитат».
type Scenario struct {
	citations CitationRepo
}

// New создаёт сценарий.
func New(citations CitationRepo) *Scenario {
	return &Scenario{citations: citations}
}

// ListCitations возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListCitations(ctx context.Context, access models.Access, page models.Page) ([]models.Citation, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeCitation, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeCitation, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.citations.ListCitations(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Citation, 0, len(list))
	for _, a := range list {
		out = append(out, *a)
	}

	return out, nil
}
