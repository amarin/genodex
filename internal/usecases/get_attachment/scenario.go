package get_attachment

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «файловое вложение по идентификатору».
type Scenario struct {
	attachments AttachmentRepo
}

// New создаёт сценарий.
func New(attachments AttachmentRepo) *Scenario {
	return &Scenario{attachments: attachments}
}

// GetAttachment возвращает вложение по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такого вложения — models.ErrNotFound. Приватное вложение
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound (docs/data-model/entity-write.md §3.1).
func (s *Scenario) GetAttachment(ctx context.Context, access models.Access, id models.ID) (models.Attachment, error) {
	if err := validateID(id); err != nil {
		return models.Attachment{}, err
	}

	a, err := s.attachments.GetAttachment(ctx, id)
	if err != nil {
		return models.Attachment{}, err
	}

	if a.Private && access != models.AccessFull {
		return models.Attachment{}, models.ErrNotFound
	}

	return *a, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeAttachment)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeAttachment,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
