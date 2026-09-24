package list_events

import (
	"context"
	"errors"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список событий».
type Scenario struct {
	events EventRepo
}

// New создаёт сценарий.
func New(events EventRepo) *Scenario {
	return &Scenario{events: events}
}

// ListEvents возвращает события, прошедшие фильтр запроса (q.PersonID, если
// задан — событие проходит, если персона участвует в нём хотя бы одной
// записью Participants), в порядке сохранения; окно применяется после
// фильтра. Некорректный запрос — *models.ValidationError, репозиторий не
// вызывается. Короткий результат (меньше размера окна) означает конец
// списка.
//
// В отличие от Relation/Residence, у Event ЕСТЬ поисковый индекс
// (search_events, по началу текста места) — этот фильтр по участнику
// дополняет поиск, а не заменяет его. Полное сканирование по generic-окнам
// ListEvents — тот же приём, что и list_relations/list_residences.
func (s *Scenario) ListEvents(ctx context.Context, access models.Access, q models.EventQuery) ([]models.Event, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	page := q.Page.Normalized()
	out := []models.Event{}
	matched := 0
	// cache — мемоизация Private по id персоны в рамках ОДНОГО вызова
	// ListEvents (не переживает вызов, не шарится между запросами): одна и
	// та же персона часто встречается среди участников нескольких событий
	// одного скана.
	cache := map[models.ID]bool{}
	// citationCache — та же мемоизация, что и cache, но для приватности
	// цитат по Sources[i].CitationID: отдельная map, независимая от cache
	// персон (см. общий паттерн citation-privacy).
	citationCache := map[models.ID]bool{}

	for offset := 0; ; offset += models.MaxPageLimit {
		list, err := s.events.ListEvents(ctx, access, models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(list) == 0 {
			return out, nil
		}

		full, nextMatched, nextOut, err := applyWindow(ctx, s.events, access, q, list, page, matched, out, cache, citationCache)
		if err != nil {
			return nil, err
		}

		matched, out = nextMatched, nextOut
		if full {
			return out, nil
		}
	}
}

// applyWindow прогоняет одно окно репозитория через фильтр запроса и
// накапливает результат окна запроса. full — окно запроса заполнено (обход
// можно остановить).
//
// Помимо q.PersonID, окно фильтрует события, ссылающиеся (через
// Participants[i].PersonID) хотя бы на одну приватную персону: для
// access != models.AccessFull такое событие прячется, даже если само оно
// не приватно (см. комментарий GetEvent). Это стоит одного дополнительного
// GetPerson на каждого участника события, попавшего в окно — приемлемо при
// масштабе этого хранилища (локальный SQLite, не высоконагруженный
// веб-сервис); тот же компромисс «без кеша/батчинга», что и в create_event
// для проверки участников.
func applyWindow(ctx context.Context, repo EventRepo, access models.Access, q models.EventQuery, list []*models.Event,
	page models.Page, matched int, out []models.Event, cache map[models.ID]bool, citationCache map[models.ID]bool,
) (full bool, nextMatched int, nextOut []models.Event, err error) {
	for _, e := range list {
		if q.PersonID != nil && !hasParticipant(e.Participants, *q.PersonID) {
			continue
		}

		if access != models.AccessFull {
			hidden, err := eventReferencesPrivatePerson(ctx, repo, e, cache)
			if err != nil {
				return false, matched, out, err
			}

			if hidden {
				continue
			}
		}

		if access != models.AccessFull {
			hidden, err := eventReferencesPrivateCitation(ctx, repo, e, citationCache)
			if err != nil {
				return false, matched, out, err
			}

			if hidden {
				continue
			}
		}

		if matched >= page.Offset {
			out = append(out, *e)

			if len(out) == page.Limit {
				return true, matched, out, nil
			}
		}

		matched++
	}

	return false, matched, out, nil
}

// eventReferencesPrivatePerson сообщает, ссылается ли событие (через
// участников Participants[i].PersonID) хотя бы на одну приватную персону.
// cache — мемоизация в рамках одного вызова ListEvents, см. её объявление
// в ListEvents. Независимая копия одноимённой функции get_event (та не
// использует cache и не глотает ErrNotFound — см. её комментарий): пакеты
// сценариев в этом проекте самодостаточны и не делятся кодом друг с
// другом.
func eventReferencesPrivatePerson(ctx context.Context, repo EventRepo, e *models.Event, cache map[models.ID]bool) (bool, error) {
	for _, p := range e.Participants {
		hidden, err := personIsPrivate(ctx, repo, cache, p.PersonID)
		if err != nil {
			return false, err
		}

		if hidden {
			return true, nil
		}
	}

	return false, nil
}

// personIsPrivate сообщает, приватна ли персона id — с точки зрения
// сканирующего ListEvents сюда же относится и гонка с конкурентным
// удалением персоны (models.ErrNotFound от GetPerson): такая персона
// трактуется как приватная, т.е. событие, ссылающееся на неё, тоже
// прячется, а не проваливает весь список ошибкой (в отличие от get_event —
// там единичный неожиданный сбой GetPerson информативнее как ошибка, а не
// как «запись не найдена»). cache — мемоизация в рамках одного вызова.
func personIsPrivate(ctx context.Context, repo EventRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
	if v, ok := cache[id]; ok {
		return v, nil
	}

	p, err := repo.GetPerson(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			cache[id] = true

			return true, nil
		}

		return false, err
	}

	cache[id] = p.Private

	return p.Private, nil
}

// eventReferencesPrivateCitation сообщает, ссылается ли событие (через
// Sources[i].CitationID) хотя бы на одну приватную цитату — независимая
// проверка, параллельная eventReferencesPrivatePerson (см. её комментарий).
// citationCache — мемоизация в рамках одного вызова ListEvents, отдельная
// от cache персон, см. комментарий её объявления в ListEvents.
func eventReferencesPrivateCitation(ctx context.Context, repo EventRepo, e *models.Event, citationCache map[models.ID]bool) (bool, error) {
	for _, sl := range e.Sources {
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
// сканирующего ListEvents сюда же относится и гонка с конкурентным
// удалением цитаты (models.ErrNotFound от GetCitation): такая цитата
// трактуется как приватная, т.е. событие, ссылающееся на неё, тоже
// прячется, а не проваливает весь список ошибкой (тот же принцип, что и у
// personIsPrivate). cache — мемоизация в рамках одного вызова.
func citationIsPrivate(ctx context.Context, repo EventRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
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

// hasParticipant сообщает, участвует ли персона id в событии.
func hasParticipant(ps []models.EventParticipant, id models.ID) bool {
	for _, p := range ps {
		if p.PersonID == id {
			return true
		}
	}

	return false
}
