package sqlstore

import (
	"errors"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/amarin/genodex/internal/idgen"
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/storage"
	"github.com/amarin/genodex/internal/store"
)

// TestTypeRegistryIsConsistent: тип модели ↔ таблица главной строки ↔
// entity_table в search_index ↔ префикс ID согласованы для всех 21 видов.
func TestTypeRegistryIsConsistent(t *testing.T) {
	s := newStore(t)

	for _, st := range fullChain() {
		mustDo(t, "save "+st.kind+" "+string(st.id), st.save(t.Context(), s))
	}

	// таблицы, реально пишущие термины: все сущности, кроме relations и residences
	indexed, err := scanRowsStrings(s, `SELECT DISTINCT entity_table FROM search_index ORDER BY entity_table`)
	mustDo(t, "search_index tables", err)

	var want []string

	for _, table := range entityTables {
		if table != "relations" && table != "residences" {
			want = append(want, table)
		}
	}

	sort.Strings(want)

	if !reflect.DeepEqual(indexed, want) {
		t.Fatalf("entity_table в search_index %v, ожидались таблицы сущностей с терминами %v", indexed, want)
	}

	gen := idgen.New()
	prefixes := map[string]models.Type{}

	for typ, table := range entityTables {
		if typeOfTable[table] != typ {
			t.Errorf("typeOfTable[%s] = %q, ожидался %q", table, typeOfTable[table], typ)
		}

		prefix := typ.IDPrefix()
		if prefix == "" {
			t.Errorf("у типа %q нет префикса ID", typ)

			continue
		}

		if other, dup := prefixes[prefix]; dup {
			t.Errorf("префикс %q у типов %q и %q", prefix, other, typ)
		}

		prefixes[prefix] = typ

		parsed, err := models.ParseID(gen.New(typ))
		if err != nil || parsed != typ {
			t.Errorf("ParseID(New(%q)) = %q, %v; ожидался тот же тип", typ, parsed, err)
		}
	}
}

// TestSaveAndDeleteMethodsAreCoveredByChain: каждому Save*/Delete*-методу
// адаптера соответствует вид из fullChain и таблица getters — новый метод без
// записи в таблицах не пройдёт.
func TestSaveAndDeleteMethodsAreCoveredByChain(t *testing.T) {
	kinds := map[string]bool{}
	for _, st := range fullChain() {
		kinds[st.kind] = true
	}

	var saves, deletes int

	typ := reflect.TypeOf(&Store{})

	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name

		switch {
		case strings.HasPrefix(name, "Save"):
			saves++

			if !kinds[strings.TrimPrefix(name, "Save")] {
				t.Errorf("метод %s не покрыт fullChain", name)
			}
		case strings.HasPrefix(name, "Delete"):
			deletes++

			if !kinds[strings.TrimPrefix(name, "Delete")] {
				t.Errorf("метод %s не покрыт fullChain", name)
			}
		}
	}

	if saves != len(kinds) || deletes != len(kinds) || len(kinds) != len(getters) || len(deleters) != len(kinds) {
		t.Fatalf("Save*=%d Delete*=%d видов в fullChain=%d getters=%d deleters=%d — ожидалось поровну",
			saves, deletes, len(kinds), len(getters), len(deleters))
	}

	for kind := range kinds {
		if getters[kind] == nil || deleters[kind] == nil {
			t.Errorf("для вида %s нет записи в getters/deleters", kind)
		}
	}
}

// TestEverySaveAndDeleteIsTransactional: внутри InTx, завершившегося ошибкой,
// все 21 Save* и все 21 Delete* не оставляют следов в таблицах, а при успехе —
// фиксируются. Метод, написанный мимо транзакции Store, повис бы на единственном
// соединении (тест ограничен по времени) или оставил бы следы.
func TestEverySaveAndDeleteIsTransactional(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	steps := fullChain()
	before := snapshot(t, s)

	saveAll := func(fail bool) error {
		return s.InTx(ctx, func(tx store.Store) error {
			scoped := tx.(*Store)

			for _, st := range steps {
				if err := st.save(ctx, scoped); err != nil {
					return err
				}
			}

			if fail {
				return errBoom
			}

			return nil
		})
	}

	deleteAll := func(fail bool) error {
		return s.InTx(ctx, func(tx store.Store) error {
			scoped := tx.(*Store)

			for i := len(steps) - 1; i >= 0; i-- {
				if err := deleters[steps[i].kind](scoped, ctx, steps[i].id); err != nil {
					return err
				}
			}

			if fail {
				return errBoom
			}

			return nil
		})
	}

	if err := saveAll(true); !errors.Is(err, errBoom) {
		t.Fatalf("saveAll(fail) = %v, ожидалась errBoom", err)
	}

	if got := snapshot(t, s); !reflect.DeepEqual(got, before) {
		t.Fatalf("откатанные Save* оставили следы:\n got  %v\n want %v", got, before)
	}

	mustDo(t, "saveAll", saveAll(false))

	full := snapshot(t, s)
	if reflect.DeepEqual(full, before) {
		t.Fatal("зафиксированные Save* ничего не записали — проверка пуста")
	}

	if err := deleteAll(true); !errors.Is(err, errBoom) {
		t.Fatalf("deleteAll(fail) = %v, ожидалась errBoom", err)
	}

	if got := snapshot(t, s); !reflect.DeepEqual(got, full) {
		t.Fatalf("откатанные Delete* изменили таблицы:\n got  %v\n want %v", got, full)
	}

	mustDo(t, "deleteAll", deleteAll(false))

	if got := snapshot(t, s); !reflect.DeepEqual(got, before) {
		t.Fatalf("зафиксированные Delete* не вернули таблицы к исходным:\n got  %v\n want %v", got, before)
	}
}

// TestListOrderSurvivesBackupAndRestore: порядок списков (rowid) после
// Backup/Restore (VACUUM INTO) тот же, даже если в rowid были дыры от удалений.
func TestListOrderSurvivesBackupAndRestore(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()

	for _, id := range []models.ID{"p-c", "p-a", "p-b", "p-d"} {
		mustDo(t, "person", s.SavePerson(ctx, &models.Person{ID: id}))
	}

	mustDo(t, "delete", s.DeletePerson(ctx, "p-a")) // дыра в rowid
	mustDo(t, "person", s.SavePerson(ctx, &models.Person{ID: "p-e"}))

	for _, id := range []models.ID{"ad-2", "ad-1", "ad-3"} {
		mustDo(t, "division", s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
			ID: id, Name: string(id), Type: models.AdminDivisionDerevnya}))
	}

	order := func(st *Store) (people, divisions []models.ID) {
		p, err := listers["People"].list(st, ctx, models.AccessFull, models.Page{})
		mustDo(t, "list people", err)

		d, err := listers["AdministrativeDivisions"].list(st, ctx, models.AccessFull, models.Page{})
		mustDo(t, "list divisions", err)

		return p, d
	}

	wantPeople, wantDivisions := order(s)
	if want := []models.ID{"p-c", "p-b", "p-d", "p-e"}; !reflect.DeepEqual(wantPeople, want) {
		t.Fatalf("порядок до бэкапа %v, ожидался %v", wantPeople, want)
	}

	if want := []models.ID{"ad-2", "ad-1", "ad-3"}; !reflect.DeepEqual(wantDivisions, want) {
		t.Fatalf("порядок делений до бэкапа %v, ожидался %v", wantDivisions, want)
	}

	dir := t.TempDir()
	bundle := filepath.Join(dir, "bundle")

	_, err := storage.Backup(s.st, bundle)
	mustDo(t, "backup", err)

	rs, err := storage.Restore(bundle, filepath.Join(dir, "restored"))
	mustDo(t, "restore", err)

	restored := New(rs)
	t.Cleanup(func() { _ = restored.Close() })

	gotPeople, gotDivisions := order(restored)

	if !reflect.DeepEqual(gotPeople, wantPeople) || !reflect.DeepEqual(gotDivisions, wantDivisions) {
		t.Fatalf("порядок после восстановления изменился:\n люди %v → %v\n деления %v → %v",
			wantPeople, gotPeople, wantDivisions, gotDivisions)
	}
}
