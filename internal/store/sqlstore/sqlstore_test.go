package sqlstore

import (
	"reflect"
	"strings"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

// newStore открывает адаптер на временном каталоге.
func newStore(t *testing.T) *Store {
	t.Helper()

	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	return s
}

// countRows считает строки произвольной таблицы: storage.DB.Count знает только
// таблицы-сущности, а тестам нужны и value-таблицы (text_refs/dates/anchors).
func countRows(t *testing.T, s *Store, table string) int {
	t.Helper()

	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}

	return n
}

// seedCitation создаёт источник и цитату — обязательную цель SourceLink
// (source_links.citation_id — строгий FK).
func seedCitation(t *testing.T, s *Store) models.ID {
	t.Helper()

	if err := s.SaveSource(t.Context(), &models.Source{
		ID: "src-1", Kind: models.SourceKindArchivalScan, Title: "МК Давыдово 1881",
	}); err != nil {
		t.Fatalf("save source: %v", err)
	}
	if err := s.SaveCitation(t.Context(), &models.Citation{ID: "cit-1", SourceID: "src-1", Text: "л. 12 об."}); err != nil {
		t.Fatalf("save citation: %v", err)
	}

	return "cit-1"
}

// TestStoreRoundTripPerson фиксирует целостность записи/чтения персоны со
// всеми вложенными списками.
func TestStoreRoundTripPerson(t *testing.T) {
	s := newStore(t)
	citID := seedCitation(t, s)

	since := models.FactDate{Year: 1881, Precision: models.PrecisionYear, Modifier: models.ModifierExact}
	want := &models.Person{
		ID:     "p-1",
		Gender: models.Male,
		Names: []models.PersonName{
			{
				Type:       models.PersonNameMain,
				Surname:    models.TextRef{Text: "Иванов", Ref: "sur-1", Type: models.TypeSurname},
				Given:      models.TextRef{Text: "Пётр"},
				Patronymic: models.TextRef{Text: "Сергеевич"},
				Prefix:     "фон",
				Suffix:     "ст.",
				Since:      &since,
			},
			{
				Type:    models.PersonNameBirth,
				Surname: models.TextRef{Text: "Петров"},
				Given:   models.TextRef{Text: "Пётр"},
			},
		},
		Estates:   []models.TextRef{{Text: "крестьянин", Ref: "est-1", Type: models.TypeEstate}},
		Titles:    []models.TextRef{{Text: "унтер-офицер"}},
		Nicknames: []models.TextRef{{Text: "Петруха"}},
		Notes:     []models.TextRef{{Text: "запись из ревизской сказки"}},
		Sources: []models.SourceLink{{
			CitationID:  citID,
			TargetType:  models.TypePerson,
			TargetID:    "p-1",
			Reliability: models.ReliabilityPrimary,
			Role:        "рождение",
			Note:        "прямое",
		}},
		Private: true,
	}

	if err := s.SavePerson(t.Context(), want); err != nil {
		t.Fatalf("save person: %v", err)
	}

	got, err := s.GetPerson(t.Context(), "p-1")
	if err != nil {
		t.Fatalf("get person: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("round trip person:\n want %+v\n got  %+v", want, got)
	}
}

// TestStoreRoundTripDivision фиксирует единицу административного деления
// со всеми видами коллекций: TextRef, строки, переименования.
func TestStoreRoundTripDivision(t *testing.T) {
	s := newStore(t)
	citID := seedCitation(t, s)

	parent := &models.AdministrativeDivision{
		ID: "ad-parent", Name: "Московская", Type: models.AdminDivisionGovernorate,
	}
	if err := s.SaveAdministrativeDivision(t.Context(), parent); err != nil {
		t.Fatalf("save parent: %v", err)
	}

	parentID := models.ID("ad-parent")
	since := models.FactDate{Year: 1700, Precision: models.PrecisionYear, Modifier: models.ModifierApprox}
	until := models.FactDate{Year: 1917, Precision: models.PrecisionYear, Modifier: models.ModifierExact}
	want := &models.AdministrativeDivision{
		ID:         "ad-1",
		Name:       "Давыдово",
		Type:       models.AdminDivisionDerevnya,
		ParentID:   &parentID,
		Items:      []models.TextRef{{Text: "двор Ивановых", Ref: "p-1", Type: models.TypePerson}},
		Variants:   []string{"Давыдово-Никольское", "Давыдовка"},
		Renames:    []models.NamedPeriod{{Text: "Давыдовка", Since: "1800", Until: "1850"}},
		Successors: []models.TextRef{{Text: "Давыдово (совр.)", Ref: "ad-9", Type: models.TypeAdministrativeDivision}},
		Since:      &since,
		Until:      &until,
		Notes:      []models.TextRef{{Text: "упомянуто в ревизии"}},
		Sources: []models.SourceLink{{
			CitationID: citID,
			TargetType: models.TypeAdministrativeDivision,
			TargetID:   "ad-1",
			Role:       "название",
		}},
	}

	if err := s.SaveAdministrativeDivision(t.Context(), want); err != nil {
		t.Fatalf("save division: %v", err)
	}

	got, err := s.GetAdministrativeDivision(t.Context(), "ad-1")
	if err != nil {
		t.Fatalf("get division: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("round trip division:\n want %+v\n got  %+v", want, got)
	}
}

// TestStoreRoundTripCitation проверяет полиморфный Anchor (дискриминатор kind).
func TestStoreRoundTripCitation(t *testing.T) {
	s := newStore(t)

	if err := s.SaveSource(t.Context(), &models.Source{ID: "src-1", Kind: models.SourceKindArchivalScan, Title: "МК"}); err != nil {
		t.Fatalf("save source: %v", err)
	}

	cases := []*models.Citation{
		{
			ID:       "cit-archive",
			SourceID: "src-1",
			Anchor:   &models.ArchiveAnchor{NodeID: "node-1", DocumentID: "doc-1", Page: 12, Rect: "10,10,100,100"},
			Text:     "родился",
			Note:     "скан",
		},
		{
			ID:       "cit-file",
			SourceID: "src-1",
			Anchor:   &models.FileAnchor{AttachmentID: "att-1", Timecode: "00:12:30"},
		},
		{
			ID:       "cit-url",
			SourceID: "src-1",
			Anchor:   &models.URLAnchor{URL: "https://example.org/page"},
			Private:  true,
		},
		{ID: "cit-plain", SourceID: "src-1", Text: "без якоря"},
	}

	for _, want := range cases {
		if err := s.SaveCitation(t.Context(), want); err != nil {
			t.Fatalf("save citation %s: %v", want.ID, err)
		}

		got, err := s.GetCitation(t.Context(), want.ID)
		if err != nil {
			t.Fatalf("get citation %s: %v", want.ID, err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("round trip citation %s:\n want %+v\n got  %+v", want.ID, want, got)
		}
	}
}

// TestStoreRoundTripEvent проверяет PlaceRef и коллекцию участников.
func TestStoreRoundTripEvent(t *testing.T) {
	s := newStore(t)
	citID := seedCitation(t, s)

	for _, id := range []models.ID{"p-1", "p-2"} {
		if err := s.SavePerson(t.Context(), &models.Person{ID: id, Gender: models.Male}); err != nil {
			t.Fatalf("save person %s: %v", id, err)
		}
	}

	date := models.FactDate{Year: 1881, Month: 3, Day: 15, Precision: models.PrecisionDay, Modifier: models.ModifierExact}
	want := &models.Event{
		ID:    "ev-1",
		Type:  models.EventTypeBirth,
		Date:  &date,
		Place: &models.PlaceRef{Text: "Давыдово", Ref: "ad-1", Type: models.TypeAdministrativeDivision},
		Participants: []models.EventParticipant{
			{PersonID: "p-1", Role: "ребёнок", Note: "первый"},
			{PersonID: "p-2", Role: "восприемник"},
		},
		Sources: []models.SourceLink{{
			CitationID: citID,
			TargetType: models.TypeEvent,
			TargetID:   "ev-1",
		}},
		Notes:   []models.TextRef{{Text: "метрическая запись"}},
		Private: true,
	}

	if err := s.SaveEvent(t.Context(), want); err != nil {
		t.Fatalf("save event: %v", err)
	}

	got, err := s.GetEvent(t.Context(), "ev-1")
	if err != nil {
		t.Fatalf("get event: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("round trip event:\n want %+v\n got  %+v", want, got)
	}
}

// TestStoreRoundTripNote проверяет заметку с иерархией «книга → глава».
func TestStoreRoundTripNote(t *testing.T) {
	s := newStore(t)
	citID := seedCitation(t, s)

	book := &models.Note{ID: "note-book", Kind: models.NoteKindBook, Title: "Род Ивановых", Text: "# Род"}
	if err := s.SaveNote(t.Context(), book); err != nil {
		t.Fatalf("save book: %v", err)
	}

	parentID := models.ID("note-book")
	want := &models.Note{
		ID:       "note-ch1",
		Kind:     models.NoteKindChapter,
		Title:    "Глава 1",
		Text:     "## Давыдово\n\nТекст.",
		ParentID: &parentID,
		Sources: []models.SourceLink{{
			CitationID: citID,
			TargetType: models.TypeNote,
			TargetID:   "note-ch1",
		}},
		Private: true,
	}

	if err := s.SaveNote(t.Context(), want); err != nil {
		t.Fatalf("save note: %v", err)
	}

	got, err := s.GetNote(t.Context(), "note-ch1")
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("round trip note:\n want %+v\n got  %+v", want, got)
	}

	list, err := s.ListNotes(t.Context())
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ожидалось 2 заметки, получено %d", len(list))
	}
}

// TestStoreRoundTripRepository проверяет хранилище-контейнер источников.
func TestStoreRoundTripRepository(t *testing.T) {
	s := newStore(t)

	want := &models.Repository{
		ID:      "rep-1",
		Name:    "ЦГА Москвы",
		Type:    models.RepositoryTypeArchive,
		Address: "Профсоюзная, 80",
		URLs:    []models.TextRef{{Text: "cgamos.ru", Ref: "", Type: ""}},
		Notes:   []models.TextRef{{Text: "читальный зал по записи"}},
		Private: false,
	}

	if err := s.SaveRepository(t.Context(), want); err != nil {
		t.Fatalf("save repository: %v", err)
	}

	got, err := s.GetRepository(t.Context(), "rep-1")
	if err != nil {
		t.Fatalf("get repository: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("round trip repository:\n want %+v\n got  %+v", want, got)
	}
}

// TestStoreRoundTripSource проверяет необязательный FK repository_id: пустой
// ID пишется как SQL NULL, заполненный — как строгая ссылка.
func TestStoreRoundTripSource(t *testing.T) {
	s := newStore(t)

	if err := s.SaveRepository(t.Context(), &models.Repository{ID: "rep-1", Name: "ЦГА Москвы"}); err != nil {
		t.Fatalf("save repository: %v", err)
	}

	date := models.FactDate{Year: 1881, Precision: models.PrecisionYear, Modifier: models.ModifierExact}
	cases := []*models.Source{
		{
			ID: "src-free", Kind: models.SourceKindMemory, Title: "Рассказ бабушки",
			Author: "А. Иванова", Reliability: models.ReliabilityMemory,
			Notes: []models.TextRef{{Text: "записано в 1990"}},
		},
		{
			ID: "src-arch", Kind: models.SourceKindArchivalScan, Title: "МК Давыдово",
			Author: "причт", Date: &date, Reliability: models.ReliabilityPrimary,
			RepositoryID: "rep-1", Private: true,
		},
	}

	for _, want := range cases {
		if err := s.SaveSource(t.Context(), want); err != nil {
			t.Fatalf("save source %s: %v", want.ID, err)
		}

		got, err := s.GetSource(t.Context(), want.ID)
		if err != nil {
			t.Fatalf("get source %s: %v", want.ID, err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("round trip source %s:\n want %+v\n got  %+v", want.ID, want, got)
		}
	}

	var nulls int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sources WHERE repository_id IS NULL`).Scan(&nulls); err != nil {
		t.Fatalf("count null repository_id: %v", err)
	}
	if nulls != 1 {
		t.Fatalf("ожидалась одна строка с repository_id IS NULL, получено %d", nulls)
	}
}

// TestStoreListPeople проверяет порядок выдачи — по порядку вставки (rowid).
func TestStoreListPeople(t *testing.T) {
	s := newStore(t)

	ids := []models.ID{"p-c", "p-a", "p-b"}
	for _, id := range ids {
		if err := s.SavePerson(t.Context(), &models.Person{ID: id, Gender: models.Unknown}); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}

	list, err := s.ListPeople(t.Context())
	if err != nil {
		t.Fatalf("list people: %v", err)
	}
	if len(list) != len(ids) {
		t.Fatalf("ожидалось %d персон, получено %d", len(ids), len(list))
	}
	for i, want := range ids {
		if list[i].ID != want {
			t.Fatalf("порядок ListPeople: позиция %d — %q, ожидалось %q", i, list[i].ID, want)
		}
	}
}

// TestStoreSearchIndex проверяет, что термины пишутся нормализованными
// (нижний регистр) и что поиск регистронезависим.
func TestStoreSearchIndex(t *testing.T) {
	s := newStore(t)

	if err := s.SavePerson(t.Context(), &models.Person{
		ID: "p-1",
		Names: []models.PersonName{{
			Surname: models.TextRef{Text: "Иванов"},
			Given:   models.TextRef{Text: "Пётр"},
		}},
	}); err != nil {
		t.Fatalf("save person: %v", err)
	}

	rows, err := s.db.Query(`SELECT term FROM search_index WHERE entity_table = 'persons' AND entity_id = 'p-1'`)
	if err != nil {
		t.Fatalf("select terms: %v", err)
	}
	var terms []string
	for rows.Next() {
		var term string
		if err := rows.Scan(&term); err != nil {
			_ = rows.Close()
			t.Fatalf("scan term: %v", err)
		}
		terms = append(terms, term)
	}
	_ = rows.Close()

	if len(terms) == 0 {
		t.Fatal("search_index пуст после SavePerson")
	}
	for _, term := range terms {
		if term != strings.ToLower(term) {
			t.Fatalf("термин %q не в нижнем регистре", term)
		}
	}

	for _, query := range []string{"ИВАНОВ", "иванов", "Иван", "ПЕТР"} {
		found, err := s.searchIDs(t.Context(), "persons", query)
		if err != nil {
			t.Fatalf("searchIDs %q: %v", query, err)
		}
		if len(found) != 1 || found[0] != "p-1" {
			t.Fatalf("searchIDs %q вернул %v, ожидалось [p-1]", query, found)
		}
	}
}

// TestStoreFKRestrict проверяет, что персону, на которую ссылается связь,
// удалить нельзя (строгая ссылка — RESTRICT).
func TestStoreFKRestrict(t *testing.T) {
	s := newStore(t)

	for _, id := range []models.ID{"p-1", "p-2"} {
		if err := s.SavePerson(t.Context(), &models.Person{ID: id}); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}
	if err := s.SaveRelation(t.Context(), &models.Relation{
		ID: "rel-1", Kind: models.RelationKindMarriage, PersonA: "p-1", PersonB: "p-2",
		Notes: []models.TextRef{{Text: "венчание"}},
	}); err != nil {
		t.Fatalf("save relation: %v", err)
	}

	_, err := s.db.Exec(`DELETE FROM persons WHERE id = 'p-1'`)
	if err == nil {
		t.Fatal("удаление персоны со связью должно быть запрещено")
	}
	if !strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		t.Fatalf("ожидалась ошибка FK, получено: %v", err)
	}

	// после удаления связи персона удаляется.
	if _, err := s.db.Exec(`DELETE FROM relations WHERE id = 'rel-1'`); err != nil {
		t.Fatalf("delete relation: %v", err)
	}
	if _, err := s.db.Exec(`DELETE FROM persons WHERE id = 'p-1'`); err != nil {
		t.Fatalf("delete person: %v", err)
	}
}

// TestStoreCascadeDelete проверяет каскад: удаление персоны без строгих ссылок
// сносит её связные строки.
func TestStoreCascadeDelete(t *testing.T) {
	s := newStore(t)

	if err := s.SavePerson(t.Context(), &models.Person{
		ID:    "p-1",
		Names: []models.PersonName{{Surname: models.TextRef{Text: "Иванов"}, Given: models.TextRef{Text: "Пётр"}}},
		Notes: []models.TextRef{{Text: "заметка"}},
	}); err != nil {
		t.Fatalf("save person: %v", err)
	}

	if got := countRows(t, s, "person_names"); got != 1 {
		t.Fatalf("person_names до удаления: %d, ожидалось 1", got)
	}
	if got := countRows(t, s, "person_notes"); got != 1 {
		t.Fatalf("person_notes до удаления: %d, ожидалось 1", got)
	}

	if _, err := s.db.Exec(`DELETE FROM persons WHERE id = 'p-1'`); err != nil {
		t.Fatalf("delete person: %v", err)
	}

	if got := countRows(t, s, "person_names"); got != 0 {
		t.Fatalf("person_names после удаления: %d, ожидалось 0", got)
	}
	if got := countRows(t, s, "person_notes"); got != 0 {
		t.Fatalf("person_notes после удаления: %d, ожидалось 0", got)
	}
}

// TestStoreCount проверяет storage.DB.Count по таблице сущностей.
func TestStoreCount(t *testing.T) {
	s := newStore(t)

	for _, id := range []models.ID{"p-1", "p-2", "p-3"} {
		if err := s.SavePerson(t.Context(), &models.Person{ID: id}); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}
	// повторное сохранение — upsert, не новая строка.
	if err := s.SavePerson(t.Context(), &models.Person{ID: "p-1", Gender: models.Female}); err != nil {
		t.Fatalf("resave: %v", err)
	}

	n, err := s.db.Count("persons")
	if err != nil {
		t.Fatalf("count persons: %v", err)
	}
	if n != 3 {
		t.Fatalf("Count(persons) = %d, ожидалось 3", n)
	}
}

// TestStoreOrphanCleanup проверяет, что перезапись сущности не копит сироты
// в общих value-таблицах: связные таблицы ссылаются на text_refs/dates по FK,
// каскад владельца их не трогает — sqlstore чистит явно.
func TestStoreOrphanCleanup(t *testing.T) {
	s := newStore(t)

	since := models.FactDate{Year: 1881, Precision: models.PrecisionYear, Modifier: models.ModifierExact}
	person := &models.Person{
		ID: "p-1",
		Names: []models.PersonName{{
			Surname: models.TextRef{Text: "Иванов"},
			Given:   models.TextRef{Text: "Пётр"},
			Since:   &since,
		}},
		Notes: []models.TextRef{{Text: "a"}, {Text: "b"}, {Text: "c"}},
	}
	if err := s.SavePerson(t.Context(), person); err != nil {
		t.Fatalf("save person: %v", err)
	}

	textsAfterFirst := countRows(t, s, "text_refs")
	datesAfterFirst := countRows(t, s, "dates")

	for i := 0; i < 5; i++ {
		person.Notes = []models.TextRef{{Text: "x"}, {Text: "y"}, {Text: "z"}}
		if err := s.SavePerson(t.Context(), person); err != nil {
			t.Fatalf("resave %d: %v", i, err)
		}
	}

	if got := countRows(t, s, "text_refs"); got != textsAfterFirst {
		t.Fatalf("text_refs выросла с %d до %d при перезаписи", textsAfterFirst, got)
	}
	if got := countRows(t, s, "dates"); got != datesAfterFirst {
		t.Fatalf("dates выросла с %d до %d при перезаписи", datesAfterFirst, got)
	}

	// укорачивание списков тоже не оставляет сирот.
	person.Notes = nil
	person.Names = nil
	if err := s.SavePerson(t.Context(), person); err != nil {
		t.Fatalf("resave empty: %v", err)
	}
	if got := countRows(t, s, "text_refs"); got != 0 {
		t.Fatalf("text_refs после очистки списков: %d, ожидалось 0", got)
	}
	if got := countRows(t, s, "dates"); got != 0 {
		t.Fatalf("dates после очистки списков: %d, ожидалось 0", got)
	}

	// то же для скалярных value-колонок главной строки (events.date_id/place_id).
	event := &models.Event{ID: "ev-1", Type: models.EventTypeBirth, Date: &since,
		Place: &models.PlaceRef{Text: "Давыдово"}}
	if err := s.SaveEvent(t.Context(), event); err != nil {
		t.Fatalf("save event: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := s.SaveEvent(t.Context(), event); err != nil {
			t.Fatalf("resave event %d: %v", i, err)
		}
	}
	if got := countRows(t, s, "dates"); got != 1 {
		t.Fatalf("dates после перезаписи события: %d, ожидалось 1", got)
	}
	if got := countRows(t, s, "text_refs"); got != 1 {
		t.Fatalf("text_refs после перезаписи события: %d, ожидалось 1", got)
	}
}

// TestStoreRoundTripArchiveChain проверяет архивную цепочку и вложение.
func TestStoreRoundTripArchiveChain(t *testing.T) {
	s := newStore(t)

	if err := s.SaveRepository(t.Context(), &models.Repository{ID: "rep-1", Name: "ЦГА Москвы"}); err != nil {
		t.Fatalf("save repository: %v", err)
	}

	archive := &models.Archive{
		ID:           "arc-1",
		Name:         "ЦГАМ",
		System:       &models.TextRef{Text: "фонд-опись-дело"},
		RepositoryID: "rep-1",
		Notes:        []models.TextRef{{Text: "оцифрован"}},
	}
	if err := s.SaveArchive(t.Context(), archive); err != nil {
		t.Fatalf("save archive: %v", err)
	}
	gotArchive, err := s.GetArchive(t.Context(), "arc-1")
	if err != nil {
		t.Fatalf("get archive: %v", err)
	}
	if !reflect.DeepEqual(archive, gotArchive) {
		t.Fatalf("round trip archive:\n want %+v\n got  %+v", archive, gotArchive)
	}

	since := models.FactDate{Year: 1880, Precision: models.PrecisionYear, Modifier: models.ModifierExact}
	node := &models.ArchiveNode{
		ID: "node-1", Type: "fund", ArchiveID: "arc-1", Label: "203", Name: "Московская духовная консистория",
		Since: &since, Parish: &models.TextRef{Text: "Никольский"},
		Settlements: []models.TextRef{{Text: "Давыдово"}},
		Notes:       []models.TextRef{{Text: "частично утрачен"}},
	}
	if err := s.SaveArchiveNode(t.Context(), node); err != nil {
		t.Fatalf("save node: %v", err)
	}
	gotNode, err := s.GetArchiveNode(t.Context(), "node-1")
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if !reflect.DeepEqual(node, gotNode) {
		t.Fatalf("round trip node:\n want %+v\n got  %+v", node, gotNode)
	}

	doc := &models.ArchiveDocument{
		ID: "doc-1", UnitID: "node-1", Title: "МК 1881", Kind: "метрическая книга",
		Since: &since, Settlements: []models.TextRef{{Text: "Давыдово"}},
	}
	if err := s.SaveArchiveDocument(t.Context(), doc); err != nil {
		t.Fatalf("save doc: %v", err)
	}
	gotDoc, err := s.GetArchiveDocument(t.Context(), "doc-1")
	if err != nil {
		t.Fatalf("get doc: %v", err)
	}
	if !reflect.DeepEqual(doc, gotDoc) {
		t.Fatalf("round trip doc:\n want %+v\n got  %+v", doc, gotDoc)
	}

	docID := models.ID("doc-1")
	attachments := []*models.Attachment{
		{ID: "att-1", Kind: models.AttachmentKindScan, URI: "file://1.jpg", Filename: "1.jpg",
			MIME: "image/jpeg", Page: 12, NodeID: "node-1", DocumentID: &docID, Note: "разворот"},
		{ID: "att-2", Kind: models.AttachmentKindPhoto, URI: "file://2.jpg", NodeID: "node-1"},
	}
	for _, want := range attachments {
		if err := s.SaveAttachment(t.Context(), want); err != nil {
			t.Fatalf("save attachment %s: %v", want.ID, err)
		}
		got, err := s.GetAttachment(t.Context(), want.ID)
		if err != nil {
			t.Fatalf("get attachment %s: %v", want.ID, err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("round trip attachment %s:\n want %+v\n got  %+v", want.ID, want, got)
		}
	}

	var nulls int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM attachments WHERE document_id IS NULL`).Scan(&nulls); err != nil {
		t.Fatalf("count null document_id: %v", err)
	}
	if nulls != 1 {
		t.Fatalf("ожидалась одна строка с document_id IS NULL, получено %d", nulls)
	}
}

// TestStoreRoundTripRest покрывает round-trip остальных сущностей порта.
func TestStoreRoundTripRest(t *testing.T) {
	s := newStore(t)

	if err := s.SavePerson(t.Context(), &models.Person{ID: "p-1"}); err != nil {
		t.Fatalf("save person: %v", err)
	}
	if err := s.SavePerson(t.Context(), &models.Person{ID: "p-2"}); err != nil {
		t.Fatalf("save person: %v", err)
	}
	if err := s.SaveAdministrativeDivision(t.Context(), &models.AdministrativeDivision{
		ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionDerevnya,
	}); err != nil {
		t.Fatalf("save division: %v", err)
	}

	since := models.FactDate{Year: 1900, Precision: models.PrecisionYear, Modifier: models.ModifierExact}

	relation := &models.Relation{
		ID: "rel-1", Kind: models.RelationKindAssociate, RelType: models.RelationTypeGodparent,
		PersonA: "p-1", PersonB: "p-2", Since: &since,
		Notes: []models.TextRef{{Text: "кум"}}, Private: true,
	}
	if err := s.SaveRelation(t.Context(), relation); err != nil {
		t.Fatalf("save relation: %v", err)
	}
	if got, err := s.GetRelation(t.Context(), "rel-1"); err != nil || !reflect.DeepEqual(relation, got) {
		t.Fatalf("round trip relation: err=%v\n want %+v\n got  %+v", err, relation, got)
	}

	residence := &models.Residence{
		ID: "res-1", PersonID: "p-1", PlaceID: "ad-1", Since: &since, Note: "по ревизии",
	}
	if err := s.SaveResidence(t.Context(), residence); err != nil {
		t.Fatalf("save residence: %v", err)
	}
	if got, err := s.GetResidence(t.Context(), "res-1"); err != nil || !reflect.DeepEqual(residence, got) {
		t.Fatalf("round trip residence: err=%v\n want %+v\n got  %+v", err, residence, got)
	}

	family := &models.Family{
		ID: "fam-1", Name: "Ивановы",
		Members: []models.TextRef{{Text: "Иванов Пётр", Ref: "p-1", Type: models.TypePerson}},
		Notes:   []models.TextRef{{Text: "линия по отцу"}},
	}
	if err := s.SaveFamily(t.Context(), family); err != nil {
		t.Fatalf("save family: %v", err)
	}
	if got, err := s.GetFamily(t.Context(), "fam-1"); err != nil || !reflect.DeepEqual(family, got) {
		t.Fatalf("round trip family: err=%v\n want %+v\n got  %+v", err, family, got)
	}

	surname := &models.Surname{
		ID: "sur-1", Canonical: "Иванов",
		Variants: []models.TextRef{{Text: "Иванофф"}},
		Items:    []models.TextRef{{Text: "Иванов Пётр", Ref: "p-1", Type: models.TypePerson}},
		Notes:    []models.TextRef{{Text: "частая фамилия"}},
	}
	if err := s.SaveSurname(t.Context(), surname); err != nil {
		t.Fatalf("save surname: %v", err)
	}
	if got, err := s.GetSurname(t.Context(), "sur-1"); err != nil || !reflect.DeepEqual(surname, got) {
		t.Fatalf("round trip surname: err=%v\n want %+v\n got  %+v", err, surname, got)
	}

	given := &models.GivenName{
		ID: "giv-1", Canonical: "Пётр", Gender: models.MaleName,
		Variants: []models.TextRef{{Text: "Петр"}},
	}
	if err := s.SaveGivenName(t.Context(), given); err != nil {
		t.Fatalf("save given name: %v", err)
	}
	if got, err := s.GetGivenName(t.Context(), "giv-1"); err != nil || !reflect.DeepEqual(given, got) {
		t.Fatalf("round trip given name: err=%v\n want %+v\n got  %+v", err, given, got)
	}

	patronymic := &models.Patronymic{ID: "pat-1", Canonical: "Сергеевич"}
	if err := s.SavePatronymic(t.Context(), patronymic); err != nil {
		t.Fatalf("save patronymic: %v", err)
	}
	if got, err := s.GetPatronymic(t.Context(), "pat-1"); err != nil || !reflect.DeepEqual(patronymic, got) {
		t.Fatalf("round trip patronymic: err=%v\n want %+v\n got  %+v", err, patronymic, got)
	}

	estate := &models.Estate{ID: "est-1", Canonical: "крестьянин",
		Variants: []models.TextRef{{Text: "крестьяне"}}}
	if err := s.SaveEstate(t.Context(), estate); err != nil {
		t.Fatalf("save estate: %v", err)
	}
	if got, err := s.GetEstate(t.Context(), "est-1"); err != nil || !reflect.DeepEqual(estate, got) {
		t.Fatalf("round trip estate: err=%v\n want %+v\n got  %+v", err, estate, got)
	}

	title := &models.Title{ID: "tit-1", Canonical: "унтер-офицер"}
	if err := s.SaveTitle(t.Context(), title); err != nil {
		t.Fatalf("save title: %v", err)
	}
	if got, err := s.GetTitle(t.Context(), "tit-1"); err != nil || !reflect.DeepEqual(title, got) {
		t.Fatalf("round trip title: err=%v\n want %+v\n got  %+v", err, title, got)
	}

	church := &models.Church{
		ID: "chu-1", Name: "Никольская",
		Parish:      &models.TextRef{Text: "Никольский приход", Ref: "par-1", Type: models.TypeParish},
		Settlements: []models.TextRef{{Text: "Давыдово", Ref: "ad-1", Type: models.TypeAdministrativeDivision}},
		Variants:    []string{"Николая Чудотворца"},
		Notes:       []models.TextRef{{Text: "деревянная"}},
	}
	if err := s.SaveChurch(t.Context(), church); err != nil {
		t.Fatalf("save church: %v", err)
	}
	if got, err := s.GetChurch(t.Context(), "chu-1"); err != nil || !reflect.DeepEqual(church, got) {
		t.Fatalf("round trip church: err=%v\n want %+v\n got  %+v", err, church, got)
	}

	parish := &models.Parish{
		ID: "par-1", Name: "Никольский",
		Church:      &models.TextRef{Text: "Никольская", Ref: "chu-1", Type: models.TypeChurch},
		Settlements: []models.TextRef{{Text: "Давыдово"}},
		Since:       &since,
	}
	if err := s.SaveParish(t.Context(), parish); err != nil {
		t.Fatalf("save parish: %v", err)
	}
	if got, err := s.GetParish(t.Context(), "par-1"); err != nil || !reflect.DeepEqual(parish, got) {
		t.Fatalf("round trip parish: err=%v\n want %+v\n got  %+v", err, parish, got)
	}

	// списки не падают и возвращают сохранённое.
	if list, err := s.ListRelations(t.Context()); err != nil || len(list) != 1 {
		t.Fatalf("ListRelations: err=%v len=%d", err, len(list))
	}
	if list, err := s.ListResidences(t.Context()); err != nil || len(list) != 1 {
		t.Fatalf("ListResidences: err=%v len=%d", err, len(list))
	}
	if list, err := s.ListFamilies(t.Context()); err != nil || len(list) != 1 {
		t.Fatalf("ListFamilies: err=%v len=%d", err, len(list))
	}
	if list, err := s.ListSurnames(t.Context()); err != nil || len(list) != 1 {
		t.Fatalf("ListSurnames: err=%v len=%d", err, len(list))
	}
	if list, err := s.ListGivenNames(t.Context()); err != nil || len(list) != 1 {
		t.Fatalf("ListGivenNames: err=%v len=%d", err, len(list))
	}
	if list, err := s.ListPatronymics(t.Context()); err != nil || len(list) != 1 {
		t.Fatalf("ListPatronymics: err=%v len=%d", err, len(list))
	}
	if list, err := s.ListEstates(t.Context()); err != nil || len(list) != 1 {
		t.Fatalf("ListEstates: err=%v len=%d", err, len(list))
	}
	if list, err := s.ListTitles(t.Context()); err != nil || len(list) != 1 {
		t.Fatalf("ListTitles: err=%v len=%d", err, len(list))
	}
	if list, err := s.ListChurches(t.Context()); err != nil || len(list) != 1 {
		t.Fatalf("ListChurches: err=%v len=%d", err, len(list))
	}
	if list, err := s.ListParishes(t.Context()); err != nil || len(list) != 1 {
		t.Fatalf("ListParishes: err=%v len=%d", err, len(list))
	}
	if list, err := s.ListAdministrativeDivisions(t.Context()); err != nil || len(list) != 1 {
		t.Fatalf("ListAdministrativeDivisions: err=%v len=%d", err, len(list))
	}
}

// Ошибка Save* называет вид и id сущности и сохраняет исходную причину (FK).
func TestSaveErrorNamesEntityAndID(t *testing.T) {
	s := newStore(t)

	err := s.SaveRelation(t.Context(), &models.Relation{
		ID: "rel-x", Kind: models.RelationKindMarriage, PersonA: "нет-1", PersonB: "нет-2",
	})
	if err == nil {
		t.Fatal("сохранение связи с несуществующими персонами должно упасть")
	}
	msg := err.Error()
	if !strings.Contains(msg, "relation") || !strings.Contains(msg, "rel-x") {
		t.Fatalf("ошибка не называет вид и id: %v", err)
	}
	if !strings.Contains(msg, "FOREIGN KEY constraint failed") {
		t.Fatalf("исходная причина потеряна: %v", err)
	}
}

// TestStoreDateCalendarRoundTrip: календарь даты пишется и читается как есть,
// включая пустое значение (пусто ≡ unknown, но идентичность сохраняется).
func TestStoreDateCalendarRoundTrip(t *testing.T) {
	s := newStore(t)

	tests := []struct {
		id       models.ID
		calendar models.FactCalendar
	}{
		{"ev-cal-julian", models.FactCalendarJulian},
		{"ev-cal-gregorian", models.FactCalendarGregorian},
		{"ev-cal-unknown", models.FactCalendarUnknown},
		{"ev-cal-empty", ""},
	}
	for _, tt := range tests {
		date := models.FactDate{
			Year: 1917, Month: 10, Day: 25,
			Precision: models.PrecisionDay, Modifier: models.ModifierExact,
			Calendar: tt.calendar,
		}
		if err := s.SaveEvent(t.Context(), &models.Event{ID: tt.id, Type: models.EventTypeBirth, Date: &date}); err != nil {
			t.Fatalf("save event %s: %v", tt.id, err)
		}

		got, err := s.GetEvent(t.Context(), tt.id)
		if err != nil {
			t.Fatalf("get event %s: %v", tt.id, err)
		}
		if got.Date == nil {
			t.Fatalf("event %s: дата потеряна", tt.id)
		}
		if got.Date.Calendar != tt.calendar {
			t.Errorf("event %s: calendar = %q, want %q", tt.id, got.Date.Calendar, tt.calendar)
		}
	}
}
