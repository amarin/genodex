package sqlstore

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

// getFunc и listFunc — методы порта в виде выражений методов, приведённые к
// одному виду: так один тест обходит все 21 сущность.
type (
	getFunc  func(*Store, context.Context, models.ID) error
	listFunc func(*Store, context.Context) error
)

func probeGet[T any](get func(*Store, context.Context, models.ID) (*T, error)) getFunc {
	return func(s *Store, ctx context.Context, id models.ID) error {
		_, err := get(s, ctx, id)

		return err
	}
}

func probeList[T any](list func(*Store, context.Context) ([]*T, error)) listFunc {
	return func(s *Store, ctx context.Context) error {
		_, err := list(s, ctx)

		return err
	}
}

var getters = map[string]getFunc{
	"Person":                 probeGet((*Store).GetPerson),
	"Relation":               probeGet((*Store).GetRelation),
	"Residence":              probeGet((*Store).GetResidence),
	"Family":                 probeGet((*Store).GetFamily),
	"Surname":                probeGet((*Store).GetSurname),
	"GivenName":              probeGet((*Store).GetGivenName),
	"Patronymic":             probeGet((*Store).GetPatronymic),
	"Estate":                 probeGet((*Store).GetEstate),
	"Title":                  probeGet((*Store).GetTitle),
	"AdministrativeDivision": probeGet((*Store).GetAdministrativeDivision),
	"Church":                 probeGet((*Store).GetChurch),
	"Parish":                 probeGet((*Store).GetParish),
	"Event":                  probeGet((*Store).GetEvent),
	"Source":                 probeGet((*Store).GetSource),
	"Archive":                probeGet((*Store).GetArchive),
	"ArchiveNode":            probeGet((*Store).GetArchiveNode),
	"ArchiveDocument":        probeGet((*Store).GetArchiveDocument),
	"Attachment":             probeGet((*Store).GetAttachment),
	"Citation":               probeGet((*Store).GetCitation),
	"Note":                   probeGet((*Store).GetNote),
	"Repository":             probeGet((*Store).GetRepository),
}

var listers = map[string]listFunc{
	"People":                  probeList((*Store).ListPeople),
	"Relations":               probeList((*Store).ListRelations),
	"Residences":              probeList((*Store).ListResidences),
	"Families":                probeList((*Store).ListFamilies),
	"Surnames":                probeList((*Store).ListSurnames),
	"GivenNames":              probeList((*Store).ListGivenNames),
	"Patronymics":             probeList((*Store).ListPatronymics),
	"Estates":                 probeList((*Store).ListEstates),
	"Titles":                  probeList((*Store).ListTitles),
	"AdministrativeDivisions": probeList((*Store).ListAdministrativeDivisions),
	"Churches":                probeList((*Store).ListChurches),
	"Parishes":                probeList((*Store).ListParishes),
	"Events":                  probeList((*Store).ListEvents),
	"Sources":                 probeList((*Store).ListSources),
	"Archives":                probeList((*Store).ListArchives),
	"ArchiveNodes":            probeList((*Store).ListArchiveNodes),
	"ArchiveDocuments":        probeList((*Store).ListArchiveDocuments),
	"Attachments":             probeList((*Store).ListAttachments),
	"Citations":               probeList((*Store).ListCitations),
	"Notes":                   probeList((*Store).ListNotes),
	"Repositories":            probeList((*Store).ListRepositories),
}

// canceled возвращает уже отменённый контекст.
func canceled(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	return ctx
}

// TestPortMethodsAreCovered не даёт добавить метод порта, не внеся его в
// таблицы обхода: число Get*/List* у адаптера должно совпасть с таблицами.
func TestPortMethodsAreCovered(t *testing.T) {
	var gets, lists int

	typ := reflect.TypeOf(&Store{})
	for i := 0; i < typ.NumMethod(); i++ {
		switch name := typ.Method(i).Name; {
		case strings.HasPrefix(name, "Get"):
			gets++
		case strings.HasPrefix(name, "List"):
			lists++
		}
	}

	if gets != len(getters) || lists != len(listers) {
		t.Fatalf("методов Get*/List* = %d/%d, в таблицах %d/%d", gets, lists, len(getters), len(listers))
	}
}

// TestGetMissingReturnsErrNotFound: для отсутствующего id каждый Get отдаёт
// models.ErrNotFound, а не (nil, nil) и не ошибку хранилища.
func TestGetMissingReturnsErrNotFound(t *testing.T) {
	s := newStore(t)

	for name, get := range getters {
		t.Run(name, func(t *testing.T) {
			if err := get(s, t.Context(), "nope"); !errors.Is(err, models.ErrNotFound) {
				t.Fatalf("Get%s(nope) = %v, ожидалось models.ErrNotFound", name, err)
			}
		})
	}
}

// TestGetCanceledContextIsNotNotFound: отменённый контекст — это ошибка
// контекста, а не «не найдено».
func TestGetCanceledContextIsNotNotFound(t *testing.T) {
	s := newStore(t)

	for name, get := range getters {
		t.Run(name, func(t *testing.T) {
			err := get(s, canceled(t), "nope")
			if !errors.Is(err, context.Canceled) || errors.Is(err, models.ErrNotFound) {
				t.Fatalf("Get%s с отменённым ctx = %v, ожидалось context.Canceled", name, err)
			}
		})
	}
}

// TestListCanceledContext: List всех сущностей прерывается отменой контекста.
func TestListCanceledContext(t *testing.T) {
	s := newStore(t)

	for name, list := range listers {
		t.Run(name, func(t *testing.T) {
			if err := list(s, canceled(t)); !errors.Is(err, context.Canceled) {
				t.Fatalf("List%s с отменённым ctx = %v, ожидалось context.Canceled", name, err)
			}
		})
	}
}

// TestSaveCanceledContextWritesNothing: отменённое сохранение возвращает
// ошибку контекста с видом сущности и ничего не пишет.
func TestSaveCanceledContextWritesNothing(t *testing.T) {
	s := newStore(t)

	err := s.SaveSurname(canceled(t), &models.Surname{ID: "sur-1", Canonical: "Блохин"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SaveSurname с отменённым ctx = %v, ожидалось context.Canceled", err)
	}

	if !strings.Contains(err.Error(), "surname") {
		t.Errorf("ошибка %q не называет вид сущности", err)
	}

	if _, err := s.GetSurname(t.Context(), "sur-1"); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("после отменённого Save Get = %v, ожидалось models.ErrNotFound", err)
	}
}

// TestListEntitiesSkipsRowDeletedMidList: строка, исчезнувшая между чтением
// id и Get, пропускается; любая другая ошибка Get прерывает список.
func TestListEntitiesSkipsRowDeletedMidList(t *testing.T) {
	s := newStore(t)

	for _, id := range []models.ID{"p-1", "p-2"} {
		if err := s.SavePerson(t.Context(), &models.Person{ID: id}); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}

	gone := func(ctx context.Context, id models.ID) (*models.Person, error) {
		if id == "p-1" {
			return nil, models.ErrNotFound
		}

		return s.GetPerson(ctx, id)
	}

	got, err := listEntities(t.Context(), s, "persons", gone)
	if err != nil || len(got) != 1 || got[0].ID != "p-2" {
		t.Fatalf("listEntities = %+v, %v; ожидалась одна персона p-2", got, err)
	}

	boom := errors.New("boom")
	failing := func(context.Context, models.ID) (*models.Person, error) { return nil, boom }

	if _, err := listEntities(t.Context(), s, "persons", failing); !errors.Is(err, boom) {
		t.Fatalf("listEntities с падающим Get = %v, ожидалась boom", err)
	}
}
