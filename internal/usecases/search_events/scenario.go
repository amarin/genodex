package search_events

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск событий».
type Scenario struct {
	events EventRepo
}

// New создаёт сценарий.
func New(events EventRepo) *Scenario {
	return &Scenario{events: events}
}

// SearchEvents находит события по началу текста места (place) — единственное
// индексируемое поле события (internal/store/sqlstore/records.go:SaveEvent,
// replaceSearchIndex(tx, "events", e.ID, map[string][]string{"place": {place}})):
// type/date/участники поиском не охвачены. Та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchEvents(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Event, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Event{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Event{}
	matched := 0
	// cache — мемоизация Private по id персоны в рамках ОДНОГО вызова
	// SearchEvents (не переживает вызов, не шарится между запросами):
	// один и тот же участник часто встречается в нескольких найденных
	// событиях одного скана.
	cache := map[models.ID]bool{}
	// citationCache — та же мемоизация, что и cache, но для приватности
	// цитат по Sources[i].CitationID: отдельная map, независимая от cache
	// персон (см. общий паттерн citation-privacy).
	citationCache := map[models.ID]bool{}

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.events.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeEvent {
				continue
			}

			// Сначала загружаем запись и проверяем приватность — и только
			// ПОТОМ считаем сдвиг (offset). Иначе скрытый (приватный или
			// исчезнувший) хит съедает часть offset-бюджета, предназначенного
			// для видимых записей: matched/offset должны отражать только то,
			// что реально попало (или могло попасть) в видимый результат,
			// иначе соседние страницы дублируют и пропускают записи (см.
			// коммит "fix: ревью — пагинация search_events").
			got, err := s.events.GetEvent(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					continue
				}

				return nil, err
			}

			// Search уже отфильтровал события с Private == true (контракт
			// generic-индекса); здесь дополнительно прячем событие, если оно
			// ссылается (через участников) на приватную персону — тот же
			// принцип, что и в get_event/list_events (см. их комментарии).
			if access != models.AccessFull {
				hidden, err := eventReferencesPrivatePerson(ctx, s.events, got, cache)
				if err != nil {
					return nil, err
				}

				if hidden {
					continue
				}
			}

			// Третьим шагом (после проверки Person, до offset-проверки) —
			// независимая проверка на приватную цитату среди источников
			// события (см. общий паттерн citation-privacy).
			if access != models.AccessFull {
				hidden, err := eventReferencesPrivateCitation(ctx, s.events, got, citationCache)
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

// eventReferencesPrivatePerson сообщает, ссылается ли событие (через
// участников Participants[i].PersonID) хотя бы на одну приватную персону.
// Независимая копия одноимённой функции get_event/list_events: пакеты
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
// сканирующего List*/Search*-сценария сюда же относится и гонка с
// конкурентным удалением персоны (models.ErrNotFound от GetPerson):
// такая персона трактуется как приватная, т.е. запись, ссылающаяся на
// неё, тоже прячется, а не проваливает весь вызов ошибкой (в отличие от
// get_event/get_relation/get_residence — там единичный неожиданный сбой
// GetPerson информативнее как ошибка, а не как «запись не найдена»).
// cache — мемоизация в рамках одного вызова, см. её объявление в
// SearchEvents.
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
// citationCache — мемоизация в рамках одного вызова SearchEvents, отдельная
// от cache персон, см. комментарий её объявления в SearchEvents.
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
// сканирующего SearchEvents сюда же относится и гонка с конкурентным
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
