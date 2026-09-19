package sqlstore

import (
	"strings"
	"testing"

	"github.com/amarin/genodex/internal/entity"
)

// TestSaveListGet проверяет цикл Save→List→Get поверх internal/storage и
// что поисковый индекс действительно пишется нормализованным.
func TestSaveListGet(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	want := &entity.Settlement{
		ID:       "sett-1",
		Name:     "Давыдово",
		Metadata: map[string]string{"volost_id": "v-1"},
	}
	if err := s.SaveSettlement(want); err != nil {
		t.Fatalf("save: %v", err)
	}

	list, err := s.ListSettlements()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].ID != want.ID || list[0].Name != want.Name {
		t.Errorf("list=%+v, want [%+v]", list, want)
	}

	got, err := s.GetSettlement(want.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || got.Name != want.Name || got.Metadata["volost_id"] != "v-1" {
		t.Errorf("get=%+v, want %+v", got, want)
	}

	if _, err := s.GetSettlement("missing"); err != nil {
		t.Errorf("get missing: unexpected error %v", err)
	}

	// поиск по нормализованному индексу: «Давыдово» → «давыдово»
	ids, err := s.st.Search(string(entity.TypeSettlement), "давыд")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(ids) != 1 || ids[0] != want.ID {
		t.Errorf("search ids=%v, want [%s]", ids, want.ID)
	}
}

// TestReopenPersists — данные переживают переоткрытие адаптера.
func TestReopenPersists(t *testing.T) {
	dir := t.TempDir()

	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := s.SaveSettlement(&entity.Settlement{ID: "sett-1", Name: "Давыдово"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = s2.Close() })

	list, err := s2.ListSettlements()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || !strings.EqualFold(list[0].Name, "Давыдово") {
		t.Errorf("list=%+v, want [Давыдово]", list)
	}
}
