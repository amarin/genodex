package list_people

import (
	"context"
	"errors"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список персон».
type Scenario struct {
	people PersonRepo
}

// New создаёт сценарий.
func New(people PersonRepo) *Scenario {
	return &Scenario{people: people}
}

// ListPeople возвращает записи в порядке сохранения, окном page. Неверные
// размер/сдвиг окна — *models.ValidationError (поля limit/offset). Короткий
// результат (меньше размера окна) означает конец списка.
//
// Запись прячется, если ссылается (Sources[i].CitationID) на приватную
// цитату — для access != models.AccessFull, даже если сама персона публична
// (см. комментарий personReferencesPrivateCitation в get_person и
// applyWindow здесь). Не путать с приватностью самой персоны (p.Private) или
// с приватностью персон, на которых ссылаются другие сущности
// (Relation/Residence/Event) — у Person нет строгой ссылки на другую Person.
// Полное сканирование по generic-окнам ListPeople — тот же приём, что и
// list_archive_nodes/list_relations: выделенный метод хранилища не оправдан
// при текущем объёме данных.
func (s *Scenario) ListPeople(ctx context.Context, access models.Access, page models.Page) ([]models.Person, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypePerson, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypePerson, Field: "offset", Reason: "не может быть отрицательным"}
	}

	page = page.Normalized()
	out := []models.Person{}
	matched := 0
	// citationCache — мемоизация Private по id цитаты в рамках ОДНОГО вызова
	// ListPeople (не переживает вызов, не шарится между запросами): одна и та
	// же цитата часто встречается в нескольких персонах одного скана.
	citationCache := map[models.ID]bool{}

	for offset := 0; ; offset += models.MaxPageLimit {
		list, err := s.people.ListPeople(ctx, access, models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(list) == 0 {
			return out, nil
		}

		var full bool

		full, matched, out, err = applyWindow(ctx, s.people, access, list, page, matched, out, citationCache)
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
func applyWindow(ctx context.Context, repo PersonRepo, access models.Access, list []*models.Person,
	page models.Page, matched int, out []models.Person, citationCache map[models.ID]bool,
) (full bool, nextMatched int, nextOut []models.Person, err error) {
	for _, p := range list {
		if access != models.AccessFull {
			hidden, err := personReferencesPrivateCitation(ctx, repo, p, citationCache)
			if err != nil {
				return false, matched, out, err
			}

			if hidden {
				continue
			}
		}

		if matched >= page.Offset {
			out = append(out, *p)

			if len(out) == page.Limit {
				return true, matched, out, nil
			}
		}

		matched++
	}

	return false, matched, out, nil
}

// personReferencesPrivateCitation сообщает, ссылается ли персона (через
// Sources[i].CitationID) хотя бы на одну приватную цитату. Независимая копия
// одноимённой функции get_person (та не использует cache — единичное
// чтение): пакеты сценариев в этом проекте самодостаточны и не делятся кодом
// друг с другом. citationCache — мемоизация в рамках одного вызова
// ListPeople, см. её объявление в ListPeople.
func personReferencesPrivateCitation(ctx context.Context, repo PersonRepo, p *models.Person, citationCache map[models.ID]bool) (bool, error) {
	for _, sl := range p.Sources {
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
// сканирующего ListPeople сюда же относится и гонка с конкурентным удалением
// цитаты (models.ErrNotFound от GetCitation): такая цитата трактуется как
// приватная, т.е. персона, ссылающаяся на неё, тоже прячется, а не
// проваливает весь список ошибкой. cache — мемоизация в рамках одного
// вызова.
func citationIsPrivate(ctx context.Context, repo PersonRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
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
