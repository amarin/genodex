package list_parishes

import (
	"context"
	"errors"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список записей приходов».
type Scenario struct {
	parishes ParishRepo
}

// New создаёт сценарий.
func New(parishes ParishRepo) *Scenario {
	return &Scenario{parishes: parishes}
}

// ListParishes возвращает записи в порядке сохранения, окном page. Неверные
// размер/сдвиг окна — *models.ValidationError (поля limit/offset). Короткий
// результат (меньше размера окна) означает конец списка. Для вызывающего
// без полного доступа запись, ссылающаяся (через Sources[i].CitationID) на
// приватную цитату, исключается из результата — у Parish нет своего
// Private, но приватная цитата в источниках прячет запись целиком (тот же
// принцип, что в get_parish); окно (размер и сдвиг) применяется после этого
// фильтра, обход идёт по окнам репозитория (см. list_archive_nodes).
func (s *Scenario) ListParishes(ctx context.Context, access models.Access, page models.Page) ([]models.Parish, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeParish, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeParish, Field: "offset", Reason: "не может быть отрицательным"}
	}

	page = page.Normalized()
	out := []models.Parish{}
	matched := 0 // сколько записей прошло фильтр (для сдвига окна)
	// cache — мемоизация Private по id цитаты в рамках ОДНОГО вызова
	// ListParishes (не переживает вызов, не шарится между запросами).
	cache := map[models.ID]bool{}

	// репозиторий отдаёт окна: обходим их до пустого или до заполнения окна запроса
	for offset := 0; ; offset += models.MaxPageLimit {
		parishes, err := s.parishes.ListParishes(ctx, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(parishes) == 0 {
			return out, nil
		}

		for _, p := range parishes {
			if access != models.AccessFull {
				hidden, err := parishReferencesPrivateCitation(ctx, s.parishes, p, cache)
				if err != nil {
					return nil, err
				}

				if hidden {
					continue
				}
			}

			if matched >= page.Offset {
				out = append(out, *p)

				if len(out) == page.Limit {
					return out, nil
				}
			}

			matched++
		}
	}
}

// parishReferencesPrivateCitation сообщает, ссылается ли запись (через
// Sources[i].CitationID) хотя бы на одну приватную цитату. cache —
// мемоизация в рамках одного вызова ListParishes, см. её объявление там.
func parishReferencesPrivateCitation(ctx context.Context, repo ParishRepo, rec *models.Parish, cache map[models.ID]bool) (bool, error) {
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

// citationIsPrivate сообщает, приватна ли цитата id; гонка с конкурентным
// удалением цитаты трактуется как приватность, см. одноимённую функцию в
// list_divisions.
func citationIsPrivate(ctx context.Context, repo ParishRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
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
