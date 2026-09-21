package sqlstore

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

// getFunc и listerSpec — методы порта в виде выражений методов, приведённые к
// одному виду: так один тест обходит все 21 сущность.
type (
	getFunc func(*Store, context.Context, models.ID) error

	// listerSpec — List*-метод с таблицей главной строки; список отдаёт id.
	listerSpec struct {
		table string
		list  func(*Store, context.Context, models.Access, models.Page) ([]models.ID, error)
	}
)

func probeGet[T any](get func(*Store, context.Context, models.ID) (*T, error)) getFunc {
	return func(s *Store, ctx context.Context, id models.ID) error {
		_, err := get(s, ctx, id)

		return err
	}
}

func probeList[T any](
	table string, list func(*Store, context.Context, models.Access, models.Page) ([]*T, error),
) listerSpec {
	return listerSpec{table: table, list: func(s *Store, ctx context.Context, a models.Access, p models.Page) ([]models.ID, error) {
		items, err := list(s, ctx, a, p)

		ids := make([]models.ID, len(items))
		for i, it := range items {
			ids[i] = models.ID(reflect.ValueOf(it).Elem().FieldByName("ID").String())
		}

		return ids, err
	}}
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

var listers = map[string]listerSpec{
	"People":                  probeList("persons", (*Store).ListPeople),
	"Relations":               probeList("relations", (*Store).ListRelations),
	"Residences":              probeList("residences", (*Store).ListResidences),
	"Families":                probeList("families", (*Store).ListFamilies),
	"Surnames":                probeList("surnames", (*Store).ListSurnames),
	"GivenNames":              probeList("given_names", (*Store).ListGivenNames),
	"Patronymics":             probeList("patronymics", (*Store).ListPatronymics),
	"Estates":                 probeList("estates", (*Store).ListEstates),
	"Titles":                  probeList("titles", (*Store).ListTitles),
	"AdministrativeDivisions": probeList("administrative_divisions", (*Store).ListAdministrativeDivisions),
	"Churches":                probeList("churches", (*Store).ListChurches),
	"Parishes":                probeList("parishes", (*Store).ListParishes),
	"Events":                  probeList("events", (*Store).ListEvents),
	"Sources":                 probeList("sources", (*Store).ListSources),
	"Archives":                probeList("archives", (*Store).ListArchives),
	"ArchiveNodes":            probeList("archive_nodes", (*Store).ListArchiveNodes),
	"ArchiveDocuments":        probeList("archive_documents", (*Store).ListArchiveDocuments),
	"Attachments":             probeList("attachments", (*Store).ListAttachments),
	"Citations":               probeList("citations", (*Store).ListCitations),
	"Notes":                   probeList("notes", (*Store).ListNotes),
	"Repositories":            probeList("repositories", (*Store).ListRepositories),
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

	for name, spec := range listers {
		t.Run(name, func(t *testing.T) {
			if _, err := spec.list(s, canceled(t), models.AccessFull, models.Page{}); !errors.Is(err, context.Canceled) {
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

	got, err := listEntities(t.Context(), s, "persons", models.AccessFull, models.Page{}, gone)
	if err != nil || len(got) != 1 || got[0].ID != "p-2" {
		t.Fatalf("listEntities = %+v, %v; ожидалась одна персона p-2", got, err)
	}

	boom := errors.New("boom")
	failing := func(context.Context, models.ID) (*models.Person, error) { return nil, boom }

	if _, err := listEntities(t.Context(), s, "persons", models.AccessFull, models.Page{}, failing); !errors.Is(err, boom) {
		t.Fatalf("listEntities с падающим Get = %v, ожидалась boom", err)
	}
}
