package delete_note

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление заметки».
type Scenario struct {
	notes NoteRepo
}

// New создаёт сценарий.
func New(notes NoteRepo) *Scenario {
	return &Scenario{notes: notes}
}

// DeleteNote удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteNote(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.notes.DeleteNote(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeNote)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeNote,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
