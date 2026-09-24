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
// models.ErrNotFound (docs/data-model/entity-write.md §3.1). Тот же принцип
// распространяется на цитаты, на которые ссылается документ через Sources:
// если хотя бы один Sources[i].CitationID указывает на приватную цитату,
// документ целиком прячется как отсутствующее, даже если Private == false
// у самого документа — иначе публичный документ выдаёт факт существования
// и id приватной цитаты.
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

	if access != models.AccessFull {
		hidden, err := archiveDocumentReferencesPrivateCitation(ctx, s.archiveDocuments, d)
		if err != nil {
			return models.ArchiveDocument{}, err
		}

		if hidden {
			return models.ArchiveDocument{}, models.ErrNotFound
		}
	}

	return *d, nil
}

// archiveDocumentReferencesPrivateCitation сообщает, ссылается ли документ
// (через Sources[i].CitationID) хотя бы на одну приватную цитату.
func archiveDocumentReferencesPrivateCitation(ctx context.Context, repo ArchiveDocumentRepo, rec *models.ArchiveDocument) (bool, error) {
	for _, sl := range rec.Sources {
		c, err := repo.GetCitation(ctx, sl.CitationID)
		if err != nil {
			return false, err
		}

		if c.Private {
			return true, nil
		}
	}

	return false, nil
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
