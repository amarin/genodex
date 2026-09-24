package list_notes

import (
	"context"
	"errors"

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

// ListNotes возвращает записи в порядке сохранения, окном page. Неверные
// размер/сдвиг окна — *models.ValidationError (поля limit/offset). Короткий
// результат (меньше размера окна) означает конец списка.
//
// У Note нет собственного поля, требующего фильтра (в отличие от
// ArchiveNode/ParentID — список плоский, без вложенности по q.ParentID),
// зато запись прячется, если ссылается (Sources[i].CitationID) на приватную
// цитату — для access != models.AccessFull, даже если сама заметка публична
// (см. комментарий noteReferencesPrivateCitation в get_note и applyWindow
// здесь). Полное сканирование по generic-окнам ListNotes — тот же приём, что
// и list_archive_nodes/list_relations: выделенный метод хранилища не
// оправдан при текущем объёме данных.
func (s *Scenario) ListNotes(ctx context.Context, access models.Access, page models.Page) ([]models.Note, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeNote, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeNote, Field: "offset", Reason: "не может быть отрицательным"}
	}

	page = page.Normalized()
	out := []models.Note{}
	matched := 0
	// citationCache — мемоизация Private по id цитаты в рамках ОДНОГО вызова
	// ListNotes (не переживает вызов, не шарится между запросами): одна и та
	// же цитата часто встречается в нескольких заметках одного скана.
	citationCache := map[models.ID]bool{}

	for offset := 0; ; offset += models.MaxPageLimit {
		list, err := s.notes.ListNotes(ctx, access, models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(list) == 0 {
			return out, nil
		}

		var full bool

		full, matched, out, err = applyWindow(ctx, s.notes, access, list, page, matched, out, citationCache)
		if err != nil {
			return nil, err
		}

		if full {
			return out, nil
		}
	}
}

// applyWindow прогоняет одно окно репозитория через фильтр приватной цитаты
// и накапливает результат окна запроса. full — окно запроса заполнено (обход
// можно остановить).
func applyWindow(ctx context.Context, repo NoteRepo, access models.Access, list []*models.Note,
	page models.Page, matched int, out []models.Note, citationCache map[models.ID]bool,
) (full bool, nextMatched int, nextOut []models.Note, err error) {
	for _, n := range list {
		if access != models.AccessFull {
			hidden, err := noteReferencesPrivateCitation(ctx, repo, n, citationCache)
			if err != nil {
				return false, matched, out, err
			}

			if hidden {
				continue
			}
		}

		if matched >= page.Offset {
			out = append(out, *n)

			if len(out) == page.Limit {
				return true, matched, out, nil
			}
		}

		matched++
	}

	return false, matched, out, nil
}

// noteReferencesPrivateCitation сообщает, ссылается ли заметка (через
// Sources[i].CitationID) хотя бы на одну приватную цитату. Независимая копия
// одноимённой функции get_note (та не использует cache — единичное чтение):
// пакеты сценариев в этом проекте самодостаточны и не делятся кодом друг с
// другом. citationCache — мемоизация в рамках одного вызова ListNotes, см.
// её объявление в ListNotes.
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
// сканирующего ListNotes сюда же относится и гонка с конкурентным удалением
// цитаты (models.ErrNotFound от GetCitation): такая цитата трактуется как
// приватная, т.е. заметка, ссылающаяся на неё, тоже прячется, а не проваливает
// весь список ошибкой. cache — мемоизация в рамках одного вызова.
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
