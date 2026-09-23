package get_archive_document

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «документ внутри единицы учёта по идентификатору».
type Scenario struct {
	archiveDocuments ArchiveDocumentRepo
}

// New создаёт сценарий.
func New(archiveDocuments ArchiveDocumentRepo) *Scenario {
	return &Scenario{archiveDocuments: archiveDocuments}
}

// GetArchiveDocument возвращает документ по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такого документа — models.ErrNotFound. Приватный документ
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound (docs/data-model/entity-write.md §3.1).
func (s *Scenario) GetArchiveDocument(ctx context.Context, access models.Access, id models.ID) (models.ArchiveDocument, error) {
	if err := validateID(id); err != nil {
		return models.ArchiveDocument{}, err
	}

	d, err := s.archiveDocuments.GetArchiveDocument(ctx, id)
	if err != nil {
		return models.ArchiveDocument{}, err
	}

	if d.Private && access != models.AccessFull {
		return models.ArchiveDocument{}, models.ErrNotFound
	}

	return *d, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeArchiveDocument)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeArchiveDocument,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
