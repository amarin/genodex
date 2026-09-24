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

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.events.GetEvent(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					matched++
					continue
				}

				return nil, err
			}

			// Search уже отфильтровал события с Private == true (контракт
			// generic-индекса); здесь дополнительно прячем событие, если оно
			// ссылается (через участников) на приватную персону — тот же
			// принцип, что и в get_event/list_events (см. их комментарии).
			if access != models.AccessFull {
				hidden, err := eventReferencesPrivatePerson(ctx, s.events, got)
				if err != nil {
					return nil, err
				}

				if hidden {
					matched++
					continue
				}
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
func eventReferencesPrivatePerson(ctx context.Context, repo EventRepo, e *models.Event) (bool, error) {
	for _, p := range e.Participants {
		person, err := repo.GetPerson(ctx, p.PersonID)
		if err != nil {
			return false, err
		}

		if person.Private {
			return true, nil
		}
	}

	return false, nil
}
