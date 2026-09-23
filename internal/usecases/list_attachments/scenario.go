package list_attachments

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список файловых вложений».
type Scenario struct {
	attachments AttachmentRepo
}

// New создаёт сценарий.
func New(attachments AttachmentRepo) *Scenario {
	return &Scenario{attachments: attachments}
}

// ListAttachments возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListAttachments(ctx context.Context, access models.Access, page models.Page) ([]models.Attachment, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeAttachment, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeAttachment, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.attachments.ListAttachments(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Attachment, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
