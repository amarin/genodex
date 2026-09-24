package list_archives

import (
	"context"
	"errors"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список записей архивов».
type Scenario struct {
	archives ArchiveRepo
}

// New создаёт сценарий.
func New(archives ArchiveRepo) *Scenario {
	return &Scenario{archives: archives}
}

// ListArchives возвращает записи в порядке сохранения, окном page, исключая
// записи, ссылающиеся (через Sources[i].CitationID) на приватную цитату —
// для вызывающего без полного доступа такая запись скрывается целиком, даже
// если сама она не приватна (см. комментарий GetArchive). Неверные
// размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
//
// Фильтр строится оконным сканированием (по образцу list_archive_nodes):
// один вызов ListArchives с чужим page нельзя сузить фильтром поверх —
// страница станет короче ожидаемого, как только фильтр начнёт что-то
// исключать.
func (s *Scenario) ListArchives(ctx context.Context, access models.Access, page models.Page) ([]models.Archive, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeArchive, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeArchive, Field: "offset", Reason: "не может быть отрицательным"}
	}

	page = page.Normalized()
	out := []models.Archive{}
	matched := 0
	// cache — мемоизация Private по id цитаты в рамках ОДНОГО вызова
	// ListArchives (не переживает вызов, не шарится между запросами).
	cache := map[models.ID]bool{}

	for offset := 0; ; offset += models.MaxPageLimit {
		list, err := s.archives.ListArchives(ctx, access, models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(list) == 0 {
			return out, nil
		}

		full, nextMatched, nextOut, err := applyWindow(ctx, s.archives, access, list, page, matched, out, cache)
		if err != nil {
			return nil, err
		}

		matched, out = nextMatched, nextOut
		if full {
			return out, nil
		}
	}
}

// applyWindow прогоняет одно окно репозитория через фильтр приватной
// цитаты и накапливает результат окна запроса. full — окно запроса
// заполнено (обход можно остановить).
func applyWindow(ctx context.Context, repo ArchiveRepo, access models.Access, list []*models.Archive,
	page models.Page, matched int, out []models.Archive, cache map[models.ID]bool,
) (full bool, nextMatched int, nextOut []models.Archive, err error) {
	for _, a := range list {
		if access != models.AccessFull {
			hidden, err := archiveReferencesPrivateCitation(ctx, repo, a, cache)
			if err != nil {
				return false, matched, out, err
			}

			if hidden {
				continue
			}
		}

		if matched >= page.Offset {
			out = append(out, *a)

			if len(out) == page.Limit {
				return true, matched, out, nil
			}
		}

		matched++
	}

	return false, matched, out, nil
}

// archiveReferencesPrivateCitation сообщает, ссылается ли архив (через
// Sources[i].CitationID) хотя бы на одну приватную цитату. Независимая
// копия одноимённой функции get_archive (та не использует cache): пакеты
// сценариев в этом проекте самодостаточны и не делятся кодом друг с
// другом. cache — мемоизация в рамках одного вызова ListArchives.
func archiveReferencesPrivateCitation(ctx context.Context, repo ArchiveRepo, rec *models.Archive, cache map[models.ID]bool) (bool, error) {
	for _, sl := range rec.Sources {
		hidden, err := citationIsPrivate(ctx, repo, cache, sl.CitationID)
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
// сканирующего List*/Search*-сценария сюда же относится и гонка с
// конкурентным удалением цитаты (models.ErrNotFound от GetCitation): такая
// цитата трактуется как приватная, т.е. запись, ссылающаяся на неё, тоже
// прячется, а не проваливает весь вызов ошибкой. cache — мемоизация в
// рамках одного вызова.
func citationIsPrivate(ctx context.Context, repo ArchiveRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
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
