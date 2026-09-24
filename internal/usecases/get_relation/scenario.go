package get_relation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «ребро графа родства по идентификатору».
type Scenario struct {
	relations RelationRepo
}

// New создаёт сценарий.
func New(relations RelationRepo) *Scenario {
	return &Scenario{relations: relations}
}

// GetRelation возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search. Тот же принцип распространяется и на персон, на которых
// ссылается ребро: если PersonA или PersonB приватны, ребро целиком
// прячется как отсутствующее, даже если Private == false у самого ребра
// (docs/data-model/entity-write.md §3.1) — иначе публичное ребро выдаёт
// сам факт существования и id приватной персоны.
func (s *Scenario) GetRelation(ctx context.Context, access models.Access, id models.ID) (models.Relation, error) {
	if err := validateID(id); err != nil {
		return models.Relation{}, err
	}

	r, err := s.relations.GetRelation(ctx, id)
	if err != nil {
		return models.Relation{}, err
	}

	if r.Private && access != models.AccessFull {
		return models.Relation{}, models.ErrNotFound
	}

	if access != models.AccessFull {
		hidden, err := relationReferencesPrivatePerson(ctx, s.relations, r)
		if err != nil {
			return models.Relation{}, err
		}

		if hidden {
			return models.Relation{}, models.ErrNotFound
		}
	}

	if access != models.AccessFull {
		hidden, err := relationReferencesPrivateCitation(ctx, s.relations, r)
		if err != nil {
			return models.Relation{}, err
		}

		if hidden {
			return models.Relation{}, models.ErrNotFound
		}
	}

	return *r, nil
}

// relationReferencesPrivatePerson сообщает, ссылается ли ребро (через
// PersonA или PersonB) на приватную персону — обе стороны проверяются
// независимо, чтобы приватность любой из них скрывала ребро целиком.
func relationReferencesPrivatePerson(ctx context.Context, repo RelationRepo, r *models.Relation) (bool, error) {
	a, err := repo.GetPerson(ctx, r.PersonA)
	if err != nil {
		return false, err
	}

	if a.Private {
		return true, nil
	}

	b, err := repo.GetPerson(ctx, r.PersonB)
	if err != nil {
		return false, err
	}

	return b.Private, nil
}

// relationReferencesPrivateCitation сообщает, ссылается ли ребро (через
// Sources[i].CitationID) хотя бы на одну приватную цитату — независимая
// проверка, параллельная relationReferencesPrivatePerson (см. её
// комментарий и комментарий GetRelation): цитата приватна независимо от
// приватности ребра и приватности персон, на которых оно ссылается.
func relationReferencesPrivateCitation(ctx context.Context, repo RelationRepo, r *models.Relation) (bool, error) {
	for _, sl := range r.Sources {
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
	err := id.Validate(models.TypeRelation)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeRelation,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
