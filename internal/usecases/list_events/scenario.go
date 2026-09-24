package list_events

import (
	"context"

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

	for offset := 0; ; offset += models.MaxPageLimit {
		list, err := s.events.ListEvents(ctx, access, models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(list) == 0 {
			return out, nil
		}

		full, nextMatched, nextOut, err := applyWindow(ctx, s.events, access, q, list, page, matched, out)
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
	page models.Page, matched int, out []models.Event,
) (full bool, nextMatched int, nextOut []models.Event, err error) {
	for _, e := range list {
		if q.PersonID != nil && !hasParticipant(e.Participants, *q.PersonID) {
			continue
		}

		if access != models.AccessFull {
			hidden, err := eventReferencesPrivatePerson(ctx, repo, e)
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
// Независимая копия одноимённой функции get_event: пакеты сценариев в этом
// проекте самодостаточны и не делятся кодом друг с другом.
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

// hasParticipant сообщает, участвует ли персона id в событии.
func hasParticipant(ps []models.EventParticipant, id models.ID) bool {
	for _, p := range ps {
		if p.PersonID == id {
			return true
		}
	}

	return false
}
