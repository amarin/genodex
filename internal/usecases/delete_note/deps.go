package delete_note

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// NoteRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type NoteRepo interface {
	DeleteNote(ctx context.Context, id models.ID) error
}
