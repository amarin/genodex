package list_attachments

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// AttachmentRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type AttachmentRepo interface {
	ListAttachments(ctx context.Context, access models.Access, page models.Page) ([]*models.Attachment, error)
}
