package list_notes

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список заметок».
type Scenario struct {
	notes NoteRepo
}

// New создаёт сценарий.
func New(notes NoteRepo) *Scenario {
	return &Scenario{notes: notes}
}

// ListNotes возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListNotes(ctx context.Context, access models.Access, page models.Page) ([]models.Note, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeNote, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeNote, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.notes.ListNotes(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Note, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
