package sqlstore

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// seedTree сохраняет дерево делений: корень ad-root с детьми ad-0003, ad-0001,
// ad-0002 (именно в таком порядке — порядок сохранения не совпадает с порядком
// id) и внуком ad-0011 у ad-0001. Единицы полностью заполнены (сверка с
// эталоном), поэтому им нужна цитата cit-1 (seedSource + SaveCitation).
func seedTree(t *testing.T, s *Store) {
	t.Helper()

	seedSource(t, s)
	mustDo(t, "citation", s.SaveCitation(t.Context(), &models.Citation{ID: "cit-1", SourceID: "src-1"}))

	root := models.ID("ad-root")
	ad1 := models.ID("ad-0001")

	mustDo(t, "root", s.SaveAdministrativeDivision(t.Context(), &models.AdministrativeDivision{
		ID: root, Name: "Московская", Type: models.AdminDivisionGovernorate,
	}))

	for _, i := range []int{3, 1, 2} {
		mustDo(t, "child", s.SaveAdministrativeDivision(t.Context(), batchDivision(i, true, &root)))
	}

	mustDo(t, "grandchild", s.SaveAdministrativeDivision(t.Context(), batchDivision(11, true, &ad1)))
}

func childIDs(t *testing.T, s *Store, parent models.ID, access models.Access, page models.Page) []models.ID {
	t.Helper()

	got, err := s.ChildrenOfDivision(t.Context(), parent, access, page)
	mustDo(t, "children of "+string(parent), err)

	if got == nil {
		t.Fatalf("ChildrenOfDivision(%s) вернул nil, ожидался пустой срез", parent)
	}

	ids := []models.ID{}
	for _, d := range got {
		ids = append(ids, d.ID)
	}

	return ids
}

// TestChildrenOfDivision: только прямые дети, в порядке сохранения, полностью
// заполненные (как поштучное чтение), листья — пустой не-nil срез.
func TestChildrenOfDivision(t *testing.T) {
	s := newStore(t)
	seedTree(t, s)

	cases := []struct {
		parent models.ID
		want   []models.ID
	}{
		{"ad-root", []models.ID{"ad-0003", "ad-0001", "ad-0002"}},
		{"ad-0001", []models.ID{"ad-0011"}},
		{"ad-0011", []models.ID{}},
		{"ad-0002", []models.ID{}},
	}

	for _, c := range cases {
		if got := childIDs(t, s, c.parent, models.AccessFull, models.Page{}); !reflect.DeepEqual(got, c.want) {
			t.Errorf("дети %s: %v, ожидалось %v", c.parent, got, c.want)
		}
	}

	children, err := s.ChildrenOfDivision(t.Context(), "ad-root", models.AccessFull, models.Page{})
	mustDo(t, "children", err)

	for _, d := range children {
		want, err := legacyGetDivision(s, t.Context(), d.ID)
		mustDo(t, "legacy "+string(d.ID), err)

		if !reflect.DeepEqual(d, want) {
			t.Fatalf("ребёнок %s расходится с поштучной загрузкой:\n got  %+v\n want %+v", d.ID, d, want)
		}
	}
}

// TestChildrenOfDivisionMissingParent: нет такого деления — ErrNotFound (а не
// пустой список: обработчик отличает «нет деления» от «нет детей»).
func TestChildrenOfDivisionMissingParent(t *testing.T) {
	s := newStore(t)

	if _, err := s.ChildrenOfDivision(t.Context(), "nope", models.AccessFull, models.Page{}); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("ChildrenOfDivision(nope) = %v, ожидалось models.ErrNotFound", err)
	}
}

// TestChildrenOfDivisionPagesAndAccess: окна не пересекаются и покрывают
// набор; режим доступа на деления не влияет.
func TestChildrenOfDivisionPagesAndAccess(t *testing.T) {
	s := newStore(t)
	seedTree(t, s)

	all := childIDs(t, s, "ad-root", models.AccessFull, models.Page{})

	var paged []models.ID

	for off := 0; off < len(all)+1; off += 2 {
		paged = append(paged, childIDs(t, s, "ad-root", models.AccessFull, models.Page{Limit: 2, Offset: off})...)
	}

	if !reflect.DeepEqual(paged, all) {
		t.Fatalf("окна по 2 в сумме %v, полный список %v", paged, all)
	}

	if got := childIDs(t, s, "ad-root", models.AccessPublic, models.Page{}); !reflect.DeepEqual(got, all) {
		t.Fatalf("публичный доступ вернул %v, ожидалось %v (у делений нет флага приватности)", got, all)
	}
}

// TestChildrenOfDivisionInTxAndCanceled: внутри InTx видны незафиксированные
// дети; отменённый контекст — ошибка контекста.
func TestChildrenOfDivisionInTxAndCanceled(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	mustDo(t, "root", s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
		ID: "ad-root", Name: "Московская", Type: models.AdminDivisionGovernorate}))

	mustDo(t, "in tx", s.InTx(ctx, func(tx store.Store) error {
		root := models.ID("ad-root")
		if err := tx.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
			ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionDerevnya, ParentID: &root}); err != nil {
			return err
		}

		got, err := tx.ChildrenOfDivision(ctx, "ad-root", models.AccessFull, models.Page{})
		if err != nil || len(got) != 1 {
			t.Errorf("внутри InTx: %v, %v; ожидался один ребёнок", got, err)
		}

		return nil
	}))

	if _, err := s.ChildrenOfDivision(canceled(t), "ad-root", models.AccessFull, models.Page{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ChildrenOfDivision с отменённым ctx = %v, ожидалось context.Canceled", err)
	}
}

// TestChildrenQueryUsesParentIndex: выборка id детей идёт по индексу parent_id
// (созданному генерацией FK-индексов) без сортировки.
func TestChildrenQueryUsesParentIndex(t *testing.T) {
	s := newStore(t)

	plan := explainDetails(t, s, pagedIDsSQL("administrative_divisions", childrenWhere), "ad-root", 50, 0)

	if !strings.Contains(plan, "idx_administrative_divisions_parent_id") {
		t.Errorf("план не использует индекс по parent_id:\n%s", plan)
	}

	if strings.Contains(plan, "TEMP B-TREE") {
		t.Errorf("план содержит сортировку:\n%s", plan)
	}
}
