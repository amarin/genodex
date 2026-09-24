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

// SearchNotes находит заметки, чей заголовок начинается с текста
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
	// citationCache — мемоизация Private по id цитаты в рамках ОДНОГО вызова
	// SearchNotes (не переживает вызов, не шарится между запросами): одна и
	// та же цитата часто встречается в нескольких найденных заметках одного
	// скана.
	citationCache := map[models.ID]bool{}

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

			// Сначала загружаем запись и проверяем приватность — и только
			// ПОТОМ считаем сдвиг (offset). Иначе скрытый (приватный или
			// исчезнувший) хит съедает часть offset-бюджета, предназначенного
			// для видимых записей (см. комментарий search_events.SearchEvents
			// и коммит "fix: ревью — пагинация search_events").
			got, err := s.notes.GetNote(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					continue
				}

				return nil, err
			}

			if access != models.AccessFull {
				hidden, err := noteReferencesPrivateCitation(ctx, s.notes, got, citationCache)
				if err != nil {
					return nil, err
				}

				if hidden {
					continue
				}
			}

			if matched < page.Offset {
				matched++

				continue
			}

			out = append(out, *got)

			if len(out) == page.Limit {
				return out, nil
			}

			matched++
		}
	}
}

// noteReferencesPrivateCitation сообщает, ссылается ли заметка (через
// Sources[i].CitationID) хотя бы на одну приватную цитату. Независимая копия
// одноимённой функции get_note/list_notes: пакеты сценариев в этом проекте
// самодостаточны и не делятся кодом друг с другом. citationCache —
// мемоизация в рамках одного вызова SearchNotes, см. её объявление в
// SearchNotes.
func noteReferencesPrivateCitation(ctx context.Context, repo NoteRepo, n *models.Note, citationCache map[models.ID]bool) (bool, error) {
	for _, sl := range n.Sources {
		hidden, err := citationIsPrivate(ctx, repo, citationCache, sl.CitationID)
		if err != nil {
			return false, err
		}

		if hidden {
			return true, nil
		}
	}

	return false, nil
}

// citationIsPrivate сообщает, приватна ли цитата id — с точки зрения
// сканирующего SearchNotes сюда же относится и гонка с конкурентным
// удалением цитаты (models.ErrNotFound от GetCitation): такая цитата
// трактуется как приватная, т.е. заметка, ссылающаяся на неё, тоже
// прячется, а не проваливает весь поиск ошибкой. cache — мемоизация в
// рамках одного вызова.
func citationIsPrivate(ctx context.Context, repo NoteRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
	if v, ok := cache[id]; ok {
		return v, nil
	}

	c, err := repo.GetCitation(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			cache[id] = true

			return true, nil
		}

		return false, err
	}

	cache[id] = c.Private

	return c.Private, nil
}
