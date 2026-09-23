package delete_attachment

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление файлового вложения».
type Scenario struct {
	attachments AttachmentRepo
}

// New создаёт сценарий.
func New(attachments AttachmentRepo) *Scenario {
	return &Scenario{attachments: attachments}
}

// DeleteAttachment удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteAttachment(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.attachments.DeleteAttachment(ctx, id)
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
