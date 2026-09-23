package search_notes

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск заметок».
type Scenario struct {
	notes NoteRepo
}

// New создаёт сценарий.
func New(notes NoteRepo) *Scenario {
	return &Scenario{notes: notes}
}

// SearchNotes находит заметки, чьи заголовок или текст начинаются с текста
// запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchNotes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Note, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Note{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Note{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.notes.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeNote {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.notes.GetNote(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					matched++
					continue
				}

				return nil, err
			}

			out = append(out, *got)

			if len(out) == page.Limit {
				return out, nil
			}

			matched++
		}
	}
}
