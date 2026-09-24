package list_churches

import (
	"context"
	"errors"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список записей церквей».
type Scenario struct {
	churches ChurchRepo
}

// New создаёт сценарий.
func New(churches ChurchRepo) *Scenario {
	return &Scenario{churches: churches}
}

// ListChurches возвращает записи в порядке сохранения, окном page. Неверные
// размер/сдвиг окна — *models.ValidationError (поля limit/offset). Короткий
// результат (меньше размера окна) означает конец списка. Для вызывающего
// без полного доступа запись, ссылающаяся (через Sources[i].CitationID) на
// приватную цитату, исключается из результата — у Church нет своего
// Private, но приватная цитата в источниках прячет запись целиком (тот же
// принцип, что в get_church); окно (размер и сдвиг) применяется после этого
// фильтра, обход идёт по окнам репозитория (см. list_archive_nodes).
func (s *Scenario) ListChurches(ctx context.Context, access models.Access, page models.Page) ([]models.Church, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeChurch, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeChurch, Field: "offset", Reason: "не может быть отрицательным"}
	}

	page = page.Normalized()
	out := []models.Church{}
	matched := 0 // сколько записей прошло фильтр (для сдвига окна)
	// cache — мемоизация Private по id цитаты в рамках ОДНОГО вызова
	// ListChurches (не переживает вызов, не шарится между запросами).
	cache := map[models.ID]bool{}

	// репозиторий отдаёт окна: обходим их до пустого или до заполнения окна запроса
	for offset := 0; ; offset += models.MaxPageLimit {
		churches, err := s.churches.ListChurches(ctx, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(churches) == 0 {
			return out, nil
		}

		for _, c := range churches {
			if access != models.AccessFull {
				hidden, err := churchReferencesPrivateCitation(ctx, s.churches, c, cache)
				if err != nil {
					return nil, err
				}

				if hidden {
					continue
				}
			}

			if matched >= page.Offset {
				out = append(out, *c)

				if len(out) == page.Limit {
					return out, nil
				}
			}

			matched++
		}
	}
}

// churchReferencesPrivateCitation сообщает, ссылается ли запись (через
// Sources[i].CitationID) хотя бы на одну приватную цитату. cache —
// мемоизация в рамках одного вызова ListChurches, см. её объявление там.
func churchReferencesPrivateCitation(ctx context.Context, repo ChurchRepo, rec *models.Church, cache map[models.ID]bool) (bool, error) {
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
func citationIsPrivate(ctx context.Context, repo ChurchRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
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
