package list_residences

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список проживаний».
type Scenario struct {
	residences ResidenceRepo
}

// New создаёт сценарий.
func New(residences ResidenceRepo) *Scenario {
	return &Scenario{residences: residences}
}

// ListResidences возвращает проживания, прошедшие фильтры запроса
// (q.PersonID/q.PlaceID, если заданы — пересекаются), в порядке сохранения;
// окно применяется после фильтра. Некорректный запрос —
// *models.ValidationError, репозиторий не вызывается. Короткий результат
// (меньше размера окна) означает конец списка.
//
// У Residence нет собственных поисковых полей (search-индекс намеренно пуст,
// docs/data-model/entity-write.md §3.8) — этот фильтр заменяет
// search_residences, которого в этой программе нет. Полное сканирование по
// generic-окнам ListResidences — тот же приём, что и list_relations/
// list_archive_nodes.
func (s *Scenario) ListResidences(ctx context.Context, access models.Access, q models.ResidenceQuery) ([]models.Residence, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	page := q.Page.Normalized()
	out := []models.Residence{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		list, err := s.residences.ListResidences(ctx, access, models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(list) == 0 {
			return out, nil
		}

		full, nextMatched, nextOut, err := applyWindow(ctx, s.residences, access, q, list, page, matched, out)
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
// Помимо q.PersonID/q.PlaceID, окно фильтрует проживания, ссылающиеся
// (PersonID) на приватную персону: для access != models.AccessFull такая
// запись прячется, даже если сама она не приватна (см. комментарий
// GetResidence). Это стоит одного дополнительного GetPerson на запись,
// попавшую в окно — приемлемо при масштабе этого хранилища (локальный
// SQLite, не высоконагруженный веб-сервис); тот же компромисс «без
// кеша/батчинга», что и в create_event для проверки участников.
func applyWindow(ctx context.Context, repo ResidenceRepo, access models.Access, q models.ResidenceQuery, list []*models.Residence,
	page models.Page, matched int, out []models.Residence,
) (full bool, nextMatched int, nextOut []models.Residence, err error) {
	for _, r := range list {
		if q.PersonID != nil && *q.PersonID != r.PersonID {
			continue
		}

		if q.PlaceID != nil && *q.PlaceID != r.PlaceID {
			continue
		}

		if access != models.AccessFull {
			hidden, err := residenceReferencesPrivatePerson(ctx, repo, r)
			if err != nil {
				return false, matched, out, err
			}

			if hidden {
				continue
			}
		}

		if matched >= page.Offset {
			out = append(out, *r)

			if len(out) == page.Limit {
				return true, matched, out, nil
			}
		}

		matched++
	}

	return false, matched, out, nil
}

// residenceReferencesPrivatePerson сообщает, ссылается ли проживание
// (через PersonID) на приватную персону. Независимая копия одноимённой
// функции get_residence: пакеты сценариев в этом проекте самодостаточны и
// не делятся кодом друг с другом.
func residenceReferencesPrivatePerson(ctx context.Context, repo ResidenceRepo, r *models.Residence) (bool, error) {
	p, err := repo.GetPerson(ctx, r.PersonID)
	if err != nil {
		return false, err
	}

	return p.Private, nil
}
