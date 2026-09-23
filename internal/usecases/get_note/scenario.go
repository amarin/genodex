package get_note

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «заметка по идентификатору».
type Scenario struct {
	notes NoteRepo
}

// New создаёт сценарий.
func New(notes NoteRepo) *Scenario {
	return &Scenario{notes: notes}
}

// GetNote возвращает заметку по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой заметки — models.ErrNotFound. Приватная заметка
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search и как исправлено для get_repository/get_archive в подпроекте 3
// (docs/data-model/entity-write.md §3.1) — Note имеет Private с самого
// начала, этот параметр не добавляется задним числом.
func (s *Scenario) GetNote(ctx context.Context, access models.Access, id models.ID) (models.Note, error) {
	if err := validateID(id); err != nil {
		return models.Note{}, err
	}

	n, err := s.notes.GetNote(ctx, id)
	if err != nil {
		return models.Note{}, err
	}

	if n.Private && access != models.AccessFull {
		return models.Note{}, models.ErrNotFound
	}

	return *n, nil
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
