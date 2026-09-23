package list_notes

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// NoteRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type NoteRepo interface {
	ListNotes(ctx context.Context, access models.Access, page models.Page) ([]*models.Note, error)
}
