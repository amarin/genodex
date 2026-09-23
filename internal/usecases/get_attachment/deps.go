package get_attachment

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// AttachmentRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type AttachmentRepo interface {
	GetAttachment(ctx context.Context, id models.ID) (*models.Attachment, error)
}
