package list_archive_documents

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список документов внутри единиц учёта».
type Scenario struct {
	archiveDocuments ArchiveDocumentRepo
}

// New создаёт сценарий.
func New(archiveDocuments ArchiveDocumentRepo) *Scenario {
	return &Scenario{archiveDocuments: archiveDocuments}
}

// ListArchiveDocuments возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка. Плоская
// сущность (в отличие от ArchiveNode) — без дополнительного фильтра.
func (s *Scenario) ListArchiveDocuments(ctx context.Context, access models.Access, page models.Page) ([]models.ArchiveDocument, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeArchiveDocument, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeArchiveDocument, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.archiveDocuments.ListArchiveDocuments(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.ArchiveDocument, 0, len(list))
	for _, d := range list {
		out = append(out, *d)
	}

	return out, nil
}
