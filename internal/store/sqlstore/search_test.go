package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func searchAll(t *testing.T, s *Store, query string, access models.Access, page models.Page) []models.Hit {
	t.Helper()

	hits, err := s.Search(t.Context(), query, access, page)
	mustDo(t, "search "+query, err)

	if hits == nil {
		t.Fatalf("Search %q вернул nil, ожидался пустой срез", query)
	}

	return hits
}

// TestSearchPrefixIsCaseAndYoInsensitive: префикс нормализуется как термины.
func TestSearchPrefixIsCaseAndYoInsensitive(t *testing.T) {
	s := newStore(t)

	mustDo(t, "person", s.SavePerson(t.Context(), &models.Person{
		ID: "p-1",
		Names: []models.PersonName{{
			Surname: models.TextRef{Text: "Иванов"}, Given: models.TextRef{Text: "Пётр"},
			Patronymic: models.TextRef{Text: "Сергеевич"},
		}},
	}))

	for _, q := range []string{"иван", "ИВАН", "Иванов", "петр", "ПЁТР", "сергеев", "  иван  "} {
		hits := searchAll(t, s, q, models.AccessFull, models.Page{})

		want := []models.Hit{{Type: models.TypePerson, ID: "p-1", Label: "Иванов Пётр Сергеевич", Field: "name"}}
		if !reflect.DeepEqual(hits, want) {
			t.Fatalf("Search %q = %+v, ожидалось %+v", q, hits, want)
		}
	}

	for _, q := range []string{"сидоров", "ванов", "иванова", "", "   "} {
		if hits := searchAll(t, s, q, models.AccessFull, models.Page{}); len(hits) != 0 {
			t.Errorf("Search %q = %+v, ожидался пустой результат", q, hits)
		}
	}
}

// TestSearchEveryIndexedKindHasLabelAndField: по одному поиску на каждый вид,
// пишущий термины в индекс (все, кроме relations и residences): тип, подпись и
// поле осмысленны.
func TestSearchEveryIndexedKindHasLabelAndField(t *testing.T) {
	s := newStore(t)

	for _, st := range fullChain() {
		mustDo(t, "save "+st.kind+" "+string(st.id), st.save(t.Context(), s))
	}

	cases := []struct {
		query string
		want  models.Hit
	}{
		{"иванов", models.Hit{Type: models.TypePerson, ID: "p-1", Label: "Иванов Пётр Сергеевич", Field: "name"}},
		{"ивановы", models.Hit{Type: models.TypeFamily, ID: "fam-1", Label: "Ивановы", Field: "name"}},
		{"иванофф", models.Hit{Type: models.TypeSurname, ID: "sur-1", Label: "Иванов", Field: "name"}},
		{"пётр", models.Hit{Type: models.TypeGivenName, ID: "giv-1", Label: "Пётр", Field: "name"}},
		{"сергиевич", models.Hit{Type: models.TypePatronymic, ID: "pat-1", Label: "Сергеевич", Field: "name"}},
		{"крестьяне", models.Hit{Type: models.TypeEstate, ID: "est-1", Label: "крестьянин", Field: "name"}},
		{"унтер-офицер", models.Hit{Type: models.TypeTitle, ID: "tit-1", Label: "унтер-офицер", Field: "name"}},
		{"давыдово", models.Hit{Type: models.TypeAdministrativeDivision, ID: "ad-1", Label: "Давыдово", Field: "name"}},
		{"никольская", models.Hit{Type: models.TypeChurch, ID: "chu-1", Label: "Никольская", Field: "name"}},
		{"никольский", models.Hit{Type: models.TypeParish, ID: "par-1", Label: "Никольский", Field: "name"}},
		{"мк давыдово", models.Hit{Type: models.TypeSource, ID: "src-1", Label: "МК Давыдово", Field: "title"}},
		{"л. 12", models.Hit{Type: models.TypeCitation, ID: "cit-1", Label: "л. 12 об.", Field: "text"}},
		{"рга", models.Hit{Type: models.TypeRepository, ID: "rep-2", Label: "РГАДА", Field: "name"}},
		{"цга москв", models.Hit{Type: models.TypeRepository, ID: "rep-1", Label: "ЦГА Москвы", Field: "name"}},
		{"цгам", models.Hit{Type: models.TypeArchive, ID: "arc-1", Label: "ЦГАМ", Field: "name"}},
		{"203", models.Hit{Type: models.TypeArchiveNode, ID: "node-1", Label: "203 Консистория", Field: "name"}},
		{"мк 1881", models.Hit{Type: models.TypeArchiveDocument, ID: "doc-1", Label: "МК 1881", Field: "title"}},
		{"1.jpg", models.Hit{Type: models.TypeAttachment, ID: "att-1", Label: "1.jpg", Field: "filename"}},
		{"род иван", models.Hit{Type: models.TypeNote, ID: "note-book", Label: "Род Ивановых", Field: "title"}},
	}

	covered := map[models.Type]bool{models.TypeEvent: true} // событие — отдельно ниже

	for _, c := range cases {
		hits := searchAll(t, s, c.query, models.AccessFull, models.Page{Limit: 100})

		found := false

		for _, h := range hits {
			found = found || h == c.want
		}

		if !found {
			t.Errorf("Search %q = %+v, не содержит %+v", c.query, hits, c.want)
		}

		covered[c.want.Type] = true
	}

	// все виды, пишущие термины в индекс, проверены (relations и residences
	// терминов не пишут)
	for _, typ := range models.AllTypes() {
		if typ == models.TypeRelation || typ == models.TypeResidence {
			continue
		}

		if !covered[typ] {
			t.Errorf("вид %s не проверен поиском", typ)
		}
	}

	// событие: подпись — тип и место
	events := searchAll(t, s, "давыд", models.AccessFull, models.Page{Limit: 100})

	wantEvent := models.Hit{Type: models.TypeEvent, ID: "ev-1", Label: "birth Давыдово", Field: "place"}

	var gotEvent bool

	for _, h := range events {
		gotEvent = gotEvent || h == wantEvent
	}

	if !gotEvent {
		t.Errorf("Search \"давыд\" = %+v, не содержит %+v", events, wantEvent)
	}
}

// TestSearchHitLabelsCoverIndexedKinds: у каждой таблицы, пишущей термины в
// индекс, есть подпись (кроме персон — своя загрузка — и таблиц без терминов).
func TestSearchHitLabelsCoverIndexedKinds(t *testing.T) {
	noTerms := map[string]bool{"relations": true, "residences": true}

	for _, table := range entityTables {
		if table == "persons" || noTerms[table] {
			continue
		}

		if _, ok := hitLabelExprs[table]; !ok {
			t.Errorf("у таблицы %s нет выражения подписи в hitLabelExprs", table)
		}
	}

	for table := range hitLabelExprs {
		if _, ok := typeOfTable[table]; !ok {
			t.Errorf("hitLabelExprs содержит %s — не таблицу сущности", table)
		}
	}
}

// TestSearchAccessHidesPrivateEntities: приватность решается по таблице
// сущности; неизвестный режим — как публичный.
func TestSearchAccessHidesPrivateEntities(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()

	mustDo(t, "p-1", s.SavePerson(ctx, &models.Person{ID: "p-1", Private: true,
		Names: []models.PersonName{{Surname: models.TextRef{Text: "Иванов"}}}}))
	mustDo(t, "p-2", s.SavePerson(ctx, &models.Person{ID: "p-2",
		Names: []models.PersonName{{Surname: models.TextRef{Text: "Иванова"}}}}))
	mustDo(t, "division", s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
		ID: "ad-1", Name: "Ивановка", Type: models.AdminDivisionDerevnya}))
	mustDo(t, "note", s.SaveNote(ctx, &models.Note{ID: "n-1", Kind: models.NoteKindBook, Title: "Иванов род", Private: true}))

	ids := func(access models.Access) []models.ID {
		var out []models.ID
		for _, h := range searchAll(t, s, "иванов", access, models.Page{}) {
			out = append(out, h.ID)
		}

		return out
	}

	cases := []struct {
		name   string
		access models.Access
		want   []models.ID
	}{
		{"полный доступ", models.AccessFull, []models.ID{"ad-1", "n-1", "p-1", "p-2"}},
		{"публичный доступ", models.AccessPublic, []models.ID{"ad-1", "p-2"}},
		{"неизвестный режим — как публичный", models.Access(9), []models.ID{"ad-1", "p-2"}},
	}

	for _, c := range cases {
		if got := ids(c.access); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, ожидалось %v", c.name, got, c.want)
		}
	}

	// окно считается после фильтра приватного
	page := searchAll(t, s, "иванов", models.AccessPublic, models.Page{Limit: 1, Offset: 1})
	if len(page) != 1 || page[0].ID != "p-2" {
		t.Errorf("второе публичное окно %+v, ожидалась p-2", page)
	}
}

// TestSearchOrderAndPagesAreStable: одна запись на сущность (два совпавших
// термина не дублируют), порядок — по виду и id, окна не пересекаются.
func TestSearchOrderAndPagesAreStable(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()

	for _, id := range []models.ID{"p-c", "p-a", "p-b"} {
		mustDo(t, "person", s.SavePerson(ctx, &models.Person{ID: id,
			Names: []models.PersonName{{Surname: models.TextRef{Text: "Иванов"}, Given: models.TextRef{Text: "Иван"}}}}))
	}

	mustDo(t, "division", s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
		ID: "ad-1", Name: "Иваново", Type: models.AdminDivisionSelo, Variants: []string{"Ивановское"}}))

	all := searchAll(t, s, "иван", models.AccessFull, models.Page{})

	var gotIDs []models.ID
	for _, h := range all {
		gotIDs = append(gotIDs, h.ID)
	}

	// «Иванов» и «Иван» у персоны — два термина, но запись одна
	if want := []models.ID{"ad-1", "p-a", "p-b", "p-c"}; !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("порядок и состав %v, ожидалось %v", gotIDs, want)
	}

	var paged []models.Hit

	for off := 0; off < len(all)+1; off++ {
		w := searchAll(t, s, "иван", models.AccessFull, models.Page{Limit: 1, Offset: off})
		paged = append(paged, w...)
	}

	if !reflect.DeepEqual(paged, all) {
		t.Fatalf("окна по одному в сумме %+v, полный список %+v", paged, all)
	}
}

// TestSearchTreatsWildcardsLiterally: %, _ и \ в запросе — обычные символы.
func TestSearchTreatsWildcardsLiterally(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()

	for id, name := range map[models.ID]string{"ad-1": "100%_вода", "ad-2": "1000 вёрст", "ad-3": `a\b`} {
		mustDo(t, "division", s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
			ID: id, Name: name, Type: models.AdminDivisionDerevnya}))
	}

	cases := map[string][]models.ID{
		"100%":  {"ad-1"},
		"100%_": {"ad-1"},
		"100_":  nil, // «_» — обычный символ, а не «любой символ»: после «100» стоит «%»
		"100":   {"ad-1", "ad-2"},
		"%":     nil,
		"_":     nil,
		`a\`:    {"ad-3"},
	}

	for q, want := range cases {
		var got []models.ID
		for _, h := range searchAll(t, s, q, models.AccessFull, models.Page{}) {
			got = append(got, h.ID)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("Search %q = %v, ожидалось %v", q, got, want)
		}
	}
}

// TestSearchFollowsSaveAndDelete: индекс обновляется при пересохранении и
// очищается при удалении; внутри InTx виден незафиксированный термин.
func TestSearchFollowsSaveAndDelete(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	mustDo(t, "save", s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
		ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionDerevnya}))
	mustDo(t, "rename", s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
		ID: "ad-1", Name: "Никифорово", Type: models.AdminDivisionDerevnya}))

	if hits := searchAll(t, s, "давыд", models.AccessFull, models.Page{}); len(hits) != 0 {
		t.Errorf("старое имя ещё находится: %+v", hits)
	}

	if hits := searchAll(t, s, "никиф", models.AccessFull, models.Page{}); len(hits) != 1 {
		t.Errorf("новое имя не находится: %+v", hits)
	}

	mustDo(t, "in tx", s.InTx(ctx, func(tx store.Store) error {
		if err := tx.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
			ID: "ad-2", Name: "Хомутово", Type: models.AdminDivisionDerevnya}); err != nil {
			return err
		}

		hits, err := tx.Search(ctx, "хому", models.AccessFull, models.Page{})
		if err != nil || len(hits) != 1 {
			t.Errorf("внутри InTx: %+v, %v; ожидалась одна запись", hits, err)
		}

		return nil
	}))

	mustDo(t, "delete", s.DeleteAdministrativeDivision(ctx, "ad-1"))

	if hits := searchAll(t, s, "никиф", models.AccessFull, models.Page{}); len(hits) != 0 {
		t.Errorf("удалённое деление находится: %+v", hits)
	}
}

// TestSearchCanceledContext: отменённый контекст — ошибка контекста.
func TestSearchCanceledContext(t *testing.T) {
	s := newStore(t)

	if _, err := s.Search(canceled(t), "иван", models.AccessFull, models.Page{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Search с отменённым ctx = %v, ожидалось context.Canceled", err)
	}
}

// explainDetails возвращает строки плана запроса.
func explainDetails(t *testing.T, s *Store, query string, args ...any) string {
	t.Helper()

	rows, err := scanRows(s.run(t.Context()), scanExplain, `EXPLAIN QUERY PLAN `+query, args...)
	mustDo(t, "explain", err)

	return strings.Join(rows, "\n")
}

// TestSearchQueryUsesTermIndex: префиксный запрос идёт по idx_search_term
// (покрывающему), а не по полному просмотру search_index.
func TestSearchQueryUsesTermIndex(t *testing.T) {
	s := newStore(t)

	g, err := s.graph(t.Context())
	mustDo(t, "graph", err)

	var private []string
	for table := range g.private {
		private = append(private, table)
	}

	sort.Strings(private)

	for name, tables := range map[string][]string{"полный доступ": nil, "публичный доступ": private} {
		plan := explainDetails(t, s, searchSQL(tables), "иван", "иван"+maxRune, 50, 0)

		if !strings.Contains(plan, "idx_search_term") {
			t.Errorf("%s: план не использует idx_search_term:\n%s", name, plan)
		}

		if !strings.Contains(plan, "COVERING INDEX idx_search_term") {
			t.Errorf("%s: индекс должен быть покрывающим:\n%s", name, plan)
		}

		if strings.Contains(plan, "SCAN si") || strings.Contains(plan, "SCAN search_index") {
			t.Errorf("%s: план содержит полный просмотр search_index:\n%s", name, plan)
		}
	}
}

// scanExplain читает столбец detail плана запроса.
func scanExplain(r *sql.Rows) (string, error) {
	var id, parent, unused int
	var detail string
	err := r.Scan(&id, &parent, &unused, &detail)

	return detail, err
}
