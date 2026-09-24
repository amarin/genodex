package get_person

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «персона по идентификатору».
type Scenario struct {
	people PersonRepo
}

// New создаёт сценарий.
func New(people PersonRepo) *Scenario {
	return &Scenario{people: people}
}

// GetPerson возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search.
func (s *Scenario) GetPerson(ctx context.Context, access models.Access, id models.ID) (models.Person, error) {
	if err := validateID(id); err != nil {
		return models.Person{}, err
	}

	p, err := s.people.GetPerson(ctx, id)
	if err != nil {
		return models.Person{}, err
	}

	if p.Private && access != models.AccessFull {
		return models.Person{}, models.ErrNotFound
	}

	if access != models.AccessFull {
		hidden, err := personReferencesPrivateCitation(ctx, s.people, p)
		if err != nil {
			return models.Person{}, err
		}

		if hidden {
			return models.Person{}, models.ErrNotFound
		}
	}

	return *p, nil
}

// personReferencesPrivateCitation сообщает, ссылается ли персона (через
// Sources[i].CitationID) хотя бы на одну приватную цитату — тот же принцип,
// что и собственный Private персоны (см. комментарий GetPerson): публичная
// персона, ссылающаяся на приватную цитату, тоже прячется как отсутствующая,
// иначе она выдаёт сам факт существования и id приватной цитаты. Не путать с
// приватностью персон, на которых ссылается Relation/Residence/Event — у
// самой Person нет строгой ссылки на другую Person, здесь речь только о
// Sources[i].CitationID.
func personReferencesPrivateCitation(ctx context.Context, repo PersonRepo, p *models.Person) (bool, error) {
	for _, sl := range p.Sources {
		c, err := repo.GetCitation(ctx, sl.CitationID)
		if err != nil {
			return false, err
		}

		if c.Private {
			return true, nil
		}
	}

	return false, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypePerson)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypePerson,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
