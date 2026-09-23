package delete_archive_document

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление документа внутри единицы учёта».
type Scenario struct {
	archiveDocuments ArchiveDocumentRepo
}

// New создаёт сценарий.
func New(archiveDocuments ArchiveDocumentRepo) *Scenario {
	return &Scenario{archiveDocuments: archiveDocuments}
}

// DeleteArchiveDocument удаляет документ. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такого
// документа — models.ErrNotFound; на документ ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteArchiveDocument(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.archiveDocuments.DeleteArchiveDocument(ctx, id)
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
