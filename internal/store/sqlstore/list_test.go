package sqlstore

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// tableHasPrivate — есть ли у таблицы колонка private (независимо от кода адаптера).
func tableHasPrivate(t *testing.T, s *Store, table string) bool {
	t.Helper()

	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = 'private'`, table).Scan(&n); err != nil {
		t.Fatalf("table_info %s: %v", table, err)
	}

	return n > 0
}

// TestListPagesPartitionEveryKind: для каждого из 21 видов окна любого размера
// не пересекаются и в сумме дают весь список, окно за пределами набора пусто,
// AccessPublic скрывает ровно приватные строки (а на таблицах без флага не
// влияет). Набор — fullChain: по сущности каждого вида, часть приватная.
func TestListPagesPartitionEveryKind(t *testing.T) {
	s := newStore(t)

	for _, st := range fullChain() {
		mustDo(t, "save "+st.kind+" "+string(st.id), st.save(t.Context(), s))
	}

	hidden := 0

	for name, spec := range listers {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()

			all, err := spec.list(s, ctx, models.AccessFull, models.Page{})
			mustDo(t, "list all", err)

			total := countRows(t, s, spec.table)
			if total == 0 || len(all) != total {
				t.Fatalf("List%s = %d строк, в таблице %d (набор должен покрывать вид)", name, len(all), total)
			}

			for _, size := range []int{1, 2, 3} {
				var got []models.ID

				// обход ограничен: сломанный сдвиг не должен зациклить тест
				for off := 0; off < total+size; off += size {
					page, err := spec.list(s, ctx, models.AccessFull, models.Page{Limit: size, Offset: off})
					mustDo(t, "list page", err)

					if len(page) > size {
						t.Fatalf("окно размера %d вернуло %d строк", size, len(page))
					}

					if len(page) == 0 {
						break
					}

					got = append(got, page...)
				}

				if !reflect.DeepEqual(got, all) {
					t.Fatalf("окна по %d в сумме %v, полный список %v", size, got, all)
				}
			}

			beyond, err := spec.list(s, ctx, models.AccessFull, models.Page{Limit: 5, Offset: total})
			mustDo(t, "list beyond", err)

			if len(beyond) != 0 {
				t.Fatalf("окно за пределами набора вернуло %v", beyond)
			}

			wantPublic := total
			if tableHasPrivate(t, s, spec.table) {
				var priv int
				if err := s.db.QueryRow(`SELECT COUNT(*) FROM ` + spec.table + ` WHERE private = 1`).Scan(&priv); err != nil {
					t.Fatalf("count private: %v", err)
				}

				wantPublic -= priv
				hidden += priv
			}

			public, err := spec.list(s, ctx, models.AccessPublic, models.Page{})
			mustDo(t, "list public", err)

			if len(public) != wantPublic {
				t.Fatalf("AccessPublic вернул %d строк, ожидалось %d", len(public), wantPublic)
			}
		})
	}

	if hidden == 0 {
		t.Fatal("в наборе нет приватных строк — проверка фильтра пуста")
	}
}

// TestListPublicHidesPrivateOnly: точный состав при разных режимах, включая
// неизвестный (он не открывает приватное).
func TestListPublicHidesPrivateOnly(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()

	mustDo(t, "p-1", s.SavePerson(ctx, &models.Person{ID: "p-1", Private: true}))
	mustDo(t, "p-2", s.SavePerson(ctx, &models.Person{ID: "p-2"}))
	mustDo(t, "p-3", s.SavePerson(ctx, &models.Person{ID: "p-3", Private: true}))
	mustDo(t, "p-4", s.SavePerson(ctx, &models.Person{ID: "p-4"}))

	ids := func(access models.Access, page models.Page) []models.ID {
		got, err := listers["People"].list(s, ctx, access, page)
		mustDo(t, "list", err)

		return got
	}

	cases := []struct {
		name   string
		access models.Access
		page   models.Page
		want   []models.ID
	}{
		{"полный доступ", models.AccessFull, models.Page{}, []models.ID{"p-1", "p-2", "p-3", "p-4"}},
		{"публичный доступ", models.AccessPublic, models.Page{}, []models.ID{"p-2", "p-4"}},
		{"неизвестный режим — как публичный", models.Access(7), models.Page{}, []models.ID{"p-2", "p-4"}},
		{"окно считается после фильтра", models.AccessPublic, models.Page{Limit: 1, Offset: 1}, []models.ID{"p-4"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ids(c.access, c.page); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("получено %v, ожидалось %v", got, c.want)
			}
		})
	}

	// у делений нет флага приватности: публичный режим отдаёт всё
	mustDo(t, "division", s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
		ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionDerevnya,
	}))

	divisions, err := listers["AdministrativeDivisions"].list(s, ctx, models.AccessPublic, models.Page{})
	mustDo(t, "list divisions", err)

	if len(divisions) != 1 {
		t.Fatalf("деления при AccessPublic: %v, ожидалась одна запись", divisions)
	}
}

// TestListDefaultAndMaxPageLimit: размер окна по умолчанию и предел.
func TestListDefaultAndMaxPageLimit(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	const total = models.MaxPageLimit + 10

	mustDo(t, "seed", s.InTx(ctx, func(tx store.Store) error {
		for i := 0; i < total; i++ {
			if err := tx.SavePerson(ctx, &models.Person{ID: models.ID("p-" + strconv.Itoa(1000+i))}); err != nil {
				return err
			}
		}

		return nil
	}))

	cases := []struct {
		name string
		page models.Page
		want int
	}{
		{"нулевое окно — 50", models.Page{}, models.DefaultPageLimit},
		{"отрицательный лимит — 50", models.Page{Limit: -5}, models.DefaultPageLimit},
		{"лимит выше предела — 500", models.Page{Limit: 1 << 20}, models.MaxPageLimit},
		{"хвост после предела", models.Page{Limit: models.MaxPageLimit, Offset: models.MaxPageLimit}, 10},
		{"отрицательный сдвиг как ноль", models.Page{Limit: 3, Offset: -4}, 3},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := listers["People"].list(s, ctx, models.AccessFull, c.page)
			mustDo(t, "list", err)

			if len(got) != c.want {
				t.Fatalf("окно %+v вернуло %d строк, ожидалось %d", c.page, len(got), c.want)
			}
		})
	}
}

// TestListOrderSurvivesResave: повторное сохранение (upsert) не меняет место
// сущности в списке — окна остаются стабильными.
func TestListOrderSurvivesResave(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()

	for _, id := range []models.ID{"p-c", "p-a", "p-b"} {
		mustDo(t, "save "+string(id), s.SavePerson(ctx, &models.Person{ID: id}))
	}

	mustDo(t, "resave", s.SavePerson(ctx, &models.Person{ID: "p-c", Gender: models.Female}))

	got, err := listers["People"].list(s, ctx, models.AccessFull, models.Page{Limit: 2, Offset: 0})
	mustDo(t, "list", err)

	if want := []models.ID{"p-c", "p-a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("первое окно после пересохранения %v, ожидалось %v", got, want)
	}
}
