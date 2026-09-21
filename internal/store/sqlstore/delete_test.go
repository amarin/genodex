package sqlstore

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type deleteFunc func(*Store, context.Context, models.ID) error

var deleters = map[string]deleteFunc{
	"Person":                 (*Store).DeletePerson,
	"Relation":               (*Store).DeleteRelation,
	"Residence":              (*Store).DeleteResidence,
	"Family":                 (*Store).DeleteFamily,
	"Surname":                (*Store).DeleteSurname,
	"GivenName":              (*Store).DeleteGivenName,
	"Patronymic":             (*Store).DeletePatronymic,
	"Estate":                 (*Store).DeleteEstate,
	"Title":                  (*Store).DeleteTitle,
	"AdministrativeDivision": (*Store).DeleteAdministrativeDivision,
	"Church":                 (*Store).DeleteChurch,
	"Parish":                 (*Store).DeleteParish,
	"Event":                  (*Store).DeleteEvent,
	"Source":                 (*Store).DeleteSource,
	"Archive":                (*Store).DeleteArchive,
	"ArchiveNode":            (*Store).DeleteArchiveNode,
	"ArchiveDocument":        (*Store).DeleteArchiveDocument,
	"Attachment":             (*Store).DeleteAttachment,
	"Citation":               (*Store).DeleteCitation,
	"Note":                   (*Store).DeleteNote,
	"Repository":             (*Store).DeleteRepository,
}

// snapshot — число строк во всех таблицах схемы (включая value-таблицы,
// search_index и source_links).
func snapshot(t *testing.T, s *Store) map[string]int {
	t.Helper()

	tables, err := scanRowsStrings(s,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("tables: %v", err)
	}

	out := make(map[string]int, len(tables))
	for _, table := range tables {
		out[table] = countRows(t, s, table)
	}

	return out
}

// link — доказательство для сущности (t, id) через цитату cit-1.
func link(t models.Type, id models.ID) []models.SourceLink {
	return []models.SourceLink{{
		CitationID: "cit-1", TargetType: t, TargetID: id,
		Reliability: models.ReliabilityPrimary, Role: "запись", Note: "прямое",
	}}
}

// chainStep — одна сущность цепочки: вид (ключ таблиц getters/deleters), id и
// сохранение полностью заполненной сущности со всеми списками, датами,
// якорем и доказательствами.
type chainStep struct {
	kind string
	id   models.ID
	save func(context.Context, *Store) error
}

// fullChain — по одной полностью заполненной сущности каждого вида. Порядок —
// порядок сохранения: каждая сущность зависит только от предыдущих.
func fullChain() []chainStep {
	since := models.FactDate{Year: 1881, Month: 3, Day: 15, Precision: models.PrecisionDay, Modifier: models.ModifierExact}
	until := models.FactDate{Year: 1917, Precision: models.PrecisionYear, Modifier: models.ModifierExact}
	docID := models.ID("doc-1")
	parentNode := models.ID("node-1")
	rootDiv := models.ID("ad-root")
	book := models.ID("note-book")
	text := func(s string) models.TextRef { return models.TextRef{Text: s} }
	ref := func(s string, id models.ID, t models.Type) models.TextRef {
		return models.TextRef{Text: s, Ref: id, Type: t}
	}

	return []chainStep{
		{"Repository", "rep-1", func(ctx context.Context, s *Store) error {
			return s.SaveRepository(ctx, &models.Repository{
				ID: "rep-1", Name: "ЦГА Москвы", Type: models.RepositoryTypeArchive, Address: "Профсоюзная, 80",
				URLs: []models.TextRef{text("cgamos.ru")}, Notes: []models.TextRef{text("читальный зал")}, Private: true,
			})
		}},
		{"Source", "src-1", func(ctx context.Context, s *Store) error {
			return s.SaveSource(ctx, &models.Source{
				ID: "src-1", Kind: models.SourceKindArchivalScan, Title: "МК Давыдово", Author: "причт",
				Date: &since, Reliability: models.ReliabilityPrimary, RepositoryID: "rep-1",
				Notes: []models.TextRef{text("скан")}, Private: true,
			})
		}},
		{"Citation", "cit-1", func(ctx context.Context, s *Store) error {
			return s.SaveCitation(ctx, &models.Citation{
				ID: "cit-1", SourceID: "src-1", Text: "л. 12 об.", Note: "запись 5",
				Anchor: &models.ArchiveAnchor{NodeID: "node-1", DocumentID: "doc-1", Page: 12, Rect: "1,1,9,9"},
			})
		}},
		{"Repository", "rep-2", func(ctx context.Context, s *Store) error {
			return s.SaveRepository(ctx, &models.Repository{
				ID: "rep-2", Name: "РГАДА", URLs: []models.TextRef{text("rgada.info")},
				Notes: []models.TextRef{text("ф. 1209")}, Sources: link(models.TypeRepository, "rep-2"),
			})
		}},
		{"Archive", "arc-1", func(ctx context.Context, s *Store) error {
			return s.SaveArchive(ctx, &models.Archive{
				ID: "arc-1", Name: "ЦГАМ", System: &models.TextRef{Text: "фонд-опись-дело"}, RepositoryID: "rep-2",
				Notes: []models.TextRef{text("оцифрован")}, Sources: link(models.TypeArchive, "arc-1"),
			})
		}},
		{"ArchiveNode", "node-1", func(ctx context.Context, s *Store) error {
			return s.SaveArchiveNode(ctx, &models.ArchiveNode{
				ID: "node-1", Type: "fund", ArchiveID: "arc-1", Label: "203", Name: "Консистория",
				Since: &since, Until: &until, Parish: &models.TextRef{Text: "Никольский"},
				Settlements: []models.TextRef{text("Давыдово")}, Notes: []models.TextRef{text("частично утрачен")},
				Sources: link(models.TypeArchiveNode, "node-1"),
			})
		}},
		{"ArchiveNode", "node-2", func(ctx context.Context, s *Store) error {
			return s.SaveArchiveNode(ctx, &models.ArchiveNode{
				ID: "node-2", Type: "inventory", ArchiveID: "arc-1", ParentID: &parentNode, Label: "3", Name: "Опись",
			})
		}},
		{"ArchiveDocument", "doc-1", func(ctx context.Context, s *Store) error {
			return s.SaveArchiveDocument(ctx, &models.ArchiveDocument{
				ID: "doc-1", UnitID: "node-2", Title: "МК 1881", Kind: "метрическая книга",
				Since: &since, Until: &until, Parish: &models.TextRef{Text: "Никольский"},
				Settlements: []models.TextRef{text("Давыдово")}, Notes: []models.TextRef{text("том 1")},
				Sources: link(models.TypeArchiveDocument, "doc-1"),
			})
		}},
		{"Attachment", "att-1", func(ctx context.Context, s *Store) error {
			return s.SaveAttachment(ctx, &models.Attachment{
				ID: "att-1", Kind: models.AttachmentKindScan, URI: "file://1.jpg", Filename: "1.jpg",
				MIME: "image/jpeg", Page: 12, NodeID: "node-2", DocumentID: &docID, Note: "разворот",
			})
		}},
		{"Surname", "sur-1", func(ctx context.Context, s *Store) error {
			return s.SaveSurname(ctx, &models.Surname{
				ID: "sur-1", Canonical: "Иванов", Variants: []models.TextRef{text("Иванофф")},
				Items: []models.TextRef{text("Иванов Пётр")}, Notes: []models.TextRef{text("частая")},
			})
		}},
		{"GivenName", "giv-1", func(ctx context.Context, s *Store) error {
			return s.SaveGivenName(ctx, &models.GivenName{
				ID: "giv-1", Canonical: "Пётр", Gender: models.MaleName,
				Variants: []models.TextRef{text("Петр")}, Items: []models.TextRef{text("Пётр Иванов")},
				Notes: []models.TextRef{text("апостол")},
			})
		}},
		{"Patronymic", "pat-1", func(ctx context.Context, s *Store) error {
			return s.SavePatronymic(ctx, &models.Patronymic{
				ID: "pat-1", Canonical: "Сергеевич", Variants: []models.TextRef{text("Сергиевич")},
				Items: []models.TextRef{text("Иванов Пётр Сергеевич")}, Notes: []models.TextRef{text("по отцу")},
			})
		}},
		{"Estate", "est-1", func(ctx context.Context, s *Store) error {
			return s.SaveEstate(ctx, &models.Estate{
				ID: "est-1", Canonical: "крестьянин", Variants: []models.TextRef{text("крестьяне")},
				Items: []models.TextRef{text("Иванов")}, Notes: []models.TextRef{text("сословие")},
			})
		}},
		{"Title", "tit-1", func(ctx context.Context, s *Store) error {
			return s.SaveTitle(ctx, &models.Title{
				ID: "tit-1", Canonical: "унтер-офицер", Variants: []models.TextRef{text("унтер")},
				Items: []models.TextRef{text("Иванов")}, Notes: []models.TextRef{text("звание")},
			})
		}},
		{"AdministrativeDivision", "ad-root", func(ctx context.Context, s *Store) error {
			return s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
				ID: "ad-root", Name: "Московская", Type: models.AdminDivisionGovernorate,
			})
		}},
		{"AdministrativeDivision", "ad-1", func(ctx context.Context, s *Store) error {
			return s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
				ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionDerevnya, ParentID: &rootDiv,
				Items:      []models.TextRef{ref("двор Ивановых", "p-1", models.TypePerson)},
				Variants:   []string{"Давыдовка"},
				Renames:    []models.NamedPeriod{{Text: "Давыдовка", Since: "1800", Until: "1850"}},
				Successors: []models.TextRef{text("Давыдово (совр.)")},
				Since:      &since, Until: &until, Notes: []models.TextRef{text("упомянуто")},
				Sources: link(models.TypeAdministrativeDivision, "ad-1"),
			})
		}},
		{"Church", "chu-1", func(ctx context.Context, s *Store) error {
			return s.SaveChurch(ctx, &models.Church{
				ID: "chu-1", Name: "Никольская", Parish: &models.TextRef{Text: "Никольский приход"},
				Settlements: []models.TextRef{text("Давыдово")}, Variants: []string{"Николая Чудотворца"},
				Notes: []models.TextRef{text("деревянная")}, Sources: link(models.TypeChurch, "chu-1"),
			})
		}},
		{"Parish", "par-1", func(ctx context.Context, s *Store) error {
			return s.SaveParish(ctx, &models.Parish{
				ID: "par-1", Name: "Никольский", Church: &models.TextRef{Text: "Никольская"},
				Settlements: []models.TextRef{text("Давыдово")}, Since: &since, Until: &until,
				Notes: []models.TextRef{text("приход")}, Sources: link(models.TypeParish, "par-1"),
			})
		}},
		{"Person", "p-1", func(ctx context.Context, s *Store) error {
			return s.SavePerson(ctx, &models.Person{
				ID: "p-1", Gender: models.PersonGenderMale,
				Names: []models.PersonName{
					{Type: models.PersonNameMain, Surname: text("Иванов"), Given: text("Пётр"),
						Patronymic: text("Сергеевич"), Prefix: "фон", Suffix: "ст.", Since: &since, Until: &until},
					{Type: models.PersonNameBirth, Surname: text("Петров"), Given: text("Пётр")},
				},
				Estates: []models.TextRef{text("крестьянин")}, Titles: []models.TextRef{text("унтер-офицер")},
				Nicknames: []models.TextRef{text("Петруха")}, Notes: []models.TextRef{text("из ревизии")},
				Sources: link(models.TypePerson, "p-1"), Private: true,
			})
		}},
		{"Person", "p-2", func(ctx context.Context, s *Store) error {
			return s.SavePerson(ctx, &models.Person{ID: "p-2", Gender: models.PersonGenderFemale})
		}},
		{"Family", "fam-1", func(ctx context.Context, s *Store) error {
			return s.SaveFamily(ctx, &models.Family{
				ID: "fam-1", Name: "Ивановы", Members: []models.TextRef{ref("Иванов Пётр", "p-1", models.TypePerson)},
				Notes: []models.TextRef{text("линия по отцу")}, Sources: link(models.TypeFamily, "fam-1"),
			})
		}},
		{"Relation", "rel-1", func(ctx context.Context, s *Store) error {
			return s.SaveRelation(ctx, &models.Relation{
				ID: "rel-1", Kind: models.RelationKindMarriage, PersonA: "p-1", PersonB: "p-2",
				Since: &since, Until: &until, Notes: []models.TextRef{text("венчание")},
				Sources: link(models.TypeRelation, "rel-1"), Private: true,
			})
		}},
		{"Residence", "res-1", func(ctx context.Context, s *Store) error {
			return s.SaveResidence(ctx, &models.Residence{
				ID: "res-1", PersonID: "p-1", PlaceID: "ad-1", Since: &since, Until: &until,
				Note: "по ревизии", Sources: link(models.TypeResidence, "res-1"),
			})
		}},
		{"Event", "ev-1", func(ctx context.Context, s *Store) error {
			return s.SaveEvent(ctx, &models.Event{
				ID: "ev-1", Type: models.EventTypeBirth, Date: &since,
				Place: &models.PlaceRef{Text: "Давыдово", Ref: "ad-1", Type: models.TypeAdministrativeDivision},
				Participants: []models.EventParticipant{
					{PersonID: "p-1", Role: "ребёнок", Note: "первый"}, {PersonID: "p-2", Role: "мать"},
				},
				Sources: link(models.TypeEvent, "ev-1"), Notes: []models.TextRef{text("метрика")}, Private: true,
			})
		}},
		{"Note", "note-book", func(ctx context.Context, s *Store) error {
			return s.SaveNote(ctx, &models.Note{
				ID: "note-book", Kind: models.NoteKindBook, Title: "Род Ивановых", Text: "# Род",
				Sources: link(models.TypeNote, "note-book"),
			})
		}},
		{"Note", "note-ch1", func(ctx context.Context, s *Store) error {
			return s.SaveNote(ctx, &models.Note{
				ID: "note-ch1", Kind: models.NoteKindChapter, Title: "Глава 1", Text: "## Давыдово",
				ParentID: &book, Sources: link(models.TypeNote, "note-ch1"), Private: true,
			})
		}},
	}
}

// TestDeleteRestoresTableCountsAtEveryStep: сущности сохраняются по одной в
// порядке зависимостей, перед каждой снимается счётчик строк всех таблиц;
// затем они удаляются в обратном порядке, и после каждого удаления счётчики
// равны снимку до её сохранения. Так проверяется всё разом: связные строки,
// search_index, source_links, text_refs, dates, anchors — и по каждой
// сущности отдельно.
func TestDeleteRestoresTableCountsAtEveryStep(t *testing.T) {
	s := newStore(t)
	steps := fullChain()
	snaps := make([]map[string]int, len(steps))

	for i, st := range steps {
		snaps[i] = snapshot(t, s)

		if err := st.save(t.Context(), s); err != nil {
			t.Fatalf("save %s %s: %v", st.kind, st.id, err)
		}

		if reflect.DeepEqual(snaps[i], snapshot(t, s)) {
			t.Fatalf("save %s %s не изменил ни одной таблицы", st.kind, st.id)
		}
	}

	if got := snapshot(t, s); got["search_index"] == 0 || got["source_links"] == 0 ||
		got["text_refs"] == 0 || got["dates"] == 0 || got["anchors"] == 0 {
		t.Fatalf("фикстура не заполнила служебные таблицы: %v", got)
	}

	for i := len(steps) - 1; i >= 0; i-- {
		st := steps[i]

		if err := deleters[st.kind](s, t.Context(), st.id); err != nil {
			t.Fatalf("delete %s %s: %v", st.kind, st.id, err)
		}

		if err := getters[st.kind](s, t.Context(), st.id); !errors.Is(err, models.ErrNotFound) {
			t.Fatalf("Get%s(%s) после удаления = %v, ожидалось ErrNotFound", st.kind, st.id, err)
		}

		if got := snapshot(t, s); !reflect.DeepEqual(got, snaps[i]) {
			t.Fatalf("после delete %s %s счётчики таблиц не вернулись к снимку до save:\n got  %v\n want %v",
				st.kind, st.id, got, snaps[i])
		}
	}
}

// TestDeleteMethodsAreCovered: число Delete*-методов адаптера совпадает с
// таблицей deleters, а та — с таблицей getters.
func TestDeleteMethodsAreCovered(t *testing.T) {
	typ := reflect.TypeOf(&Store{})

	var n int

	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		if strings.HasPrefix(name, "Delete") {
			n++

			if _, ok := deleters[strings.TrimPrefix(name, "Delete")]; !ok {
				t.Errorf("метод %s отсутствует в таблице deleters", name)
			}
		}
	}

	if n != len(deleters) || len(deleters) != len(getters) {
		t.Fatalf("методов Delete* %d, в deleters %d, в getters %d", n, len(deleters), len(getters))
	}
}

// TestDeleteMissingReturnsErrNotFound: удаление отсутствующей сущности —
// models.ErrNotFound для всех 21 видов.
func TestDeleteMissingReturnsErrNotFound(t *testing.T) {
	s := newStore(t)

	for kind, del := range deleters {
		t.Run(kind, func(t *testing.T) {
			if err := del(s, t.Context(), "nope"); !errors.Is(err, models.ErrNotFound) {
				t.Fatalf("Delete%s(nope) = %v, ожидалось models.ErrNotFound", kind, err)
			}
		})
	}
}

// TestDeleteCanceledContext: отменённый контекст — ошибка контекста, сущность
// остаётся на месте.
func TestDeleteCanceledContext(t *testing.T) {
	s := newStore(t)

	for _, id := range []models.ID{"p-1", "p-warm"} {
		if err := s.SavePerson(t.Context(), &models.Person{ID: id}); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	// прогреть кэш графа схемы: дальше отмена ловится уже внутри транзакции
	if err := s.DeletePerson(t.Context(), "p-warm"); err != nil {
		t.Fatalf("прогревающее удаление: %v", err)
	}

	before := snapshot(t, s)

	err := s.DeletePerson(canceled(t), "p-1")
	if !errors.Is(err, context.Canceled) || errors.Is(err, models.ErrNotFound) {
		t.Fatalf("DeletePerson с отменённым ctx = %v, ожидалось context.Canceled", err)
	}

	if !strings.Contains(err.Error(), `delete person "p-1"`) {
		t.Errorf("ошибка сбоя %q не называет вид и id сущности", err)
	}

	if _, err := s.GetPerson(t.Context(), "p-1"); err != nil {
		t.Fatalf("персона пропала после отменённого удаления: %v", err)
	}

	if got := snapshot(t, s); !reflect.DeepEqual(got, before) {
		t.Fatalf("отменённое удаление изменило таблицы:\n got  %v\n want %v", got, before)
	}
}

// inUseCase — удаление (kind, id) при готовых зависимостях запрещено, а после
// снятия ссылок (release) проходит.
type inUseCase struct {
	name      string
	setup     func(t *testing.T, s *Store)
	kind      string
	id        models.ID
	typ       models.Type
	referrers []models.EntityRef
	release   func(t *testing.T, s *Store)
}

func mustDo(t *testing.T, what string, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

func TestDeleteInUse(t *testing.T) {
	persons := func(t *testing.T, s *Store, ids ...models.ID) {
		t.Helper()

		for _, id := range ids {
			mustDo(t, "save person "+string(id), s.SavePerson(t.Context(), &models.Person{ID: id}))
		}
	}
	division := func(t *testing.T, s *Store, id models.ID, parent *models.ID) {
		t.Helper()
		mustDo(t, "save division "+string(id), s.SaveAdministrativeDivision(t.Context(),
			&models.AdministrativeDivision{ID: id, Name: string(id), Type: models.AdminDivisionDerevnya, ParentID: parent}))
	}
	repo := func(t *testing.T, s *Store, id models.ID) {
		t.Helper()
		mustDo(t, "save repository "+string(id), s.SaveRepository(t.Context(), &models.Repository{ID: id, Name: "Р"}))
	}
	archiveChain := func(t *testing.T, s *Store) {
		t.Helper()
		repo(t, s, "rep-1")
		mustDo(t, "archive", s.SaveArchive(t.Context(), &models.Archive{ID: "arc-1", Name: "А", RepositoryID: "rep-1"}))
		mustDo(t, "node", s.SaveArchiveNode(t.Context(), &models.ArchiveNode{ID: "node-1", Type: "fund", ArchiveID: "arc-1"}))
	}
	ref := func(t models.Type, id models.ID) models.EntityRef { return models.EntityRef{Type: t, ID: id} }

	cases := []inUseCase{
		{
			name: "персона со связью", kind: "Person", id: "p-1", typ: models.TypePerson,
			setup: func(t *testing.T, s *Store) {
				persons(t, s, "p-1", "p-2")
				mustDo(t, "relation", s.SaveRelation(t.Context(), &models.Relation{
					ID: "rel-1", Kind: models.RelationKindMarriage, PersonA: "p-1", PersonB: "p-2"}))
			},
			referrers: []models.EntityRef{ref(models.TypeRelation, "rel-1")},
			release:   func(t *testing.T, s *Store) { mustDo(t, "rel", s.DeleteRelation(t.Context(), "rel-1")) },
		},
		{
			name: "персона — участник события (связная таблица → владелец)", kind: "Person", id: "p-1", typ: models.TypePerson,
			setup: func(t *testing.T, s *Store) {
				persons(t, s, "p-1")
				mustDo(t, "event", s.SaveEvent(t.Context(), &models.Event{ID: "ev-1", Type: models.EventTypeBirth,
					Participants: []models.EventParticipant{{PersonID: "p-1"}, {PersonID: "p-1", Role: "восприемник"}}}))
			},
			referrers: []models.EntityRef{ref(models.TypeEvent, "ev-1")}, // событие один раз, хоть участий два
			release:   func(t *testing.T, s *Store) { mustDo(t, "ev", s.DeleteEvent(t.Context(), "ev-1")) },
		},
		{
			name: "персона с проживанием", kind: "Person", id: "p-1", typ: models.TypePerson,
			setup: func(t *testing.T, s *Store) {
				persons(t, s, "p-1")
				division(t, s, "ad-1", nil)
				mustDo(t, "residence", s.SaveResidence(t.Context(), &models.Residence{ID: "res-1", PersonID: "p-1", PlaceID: "ad-1"}))
			},
			referrers: []models.EntityRef{ref(models.TypeResidence, "res-1")},
			release:   func(t *testing.T, s *Store) { mustDo(t, "res", s.DeleteResidence(t.Context(), "res-1")) },
		},
		{
			name: "деление с дочерним и проживанием", kind: "AdministrativeDivision", id: "ad-1", typ: models.TypeAdministrativeDivision,
			setup: func(t *testing.T, s *Store) {
				parent := models.ID("ad-1")
				division(t, s, "ad-1", nil)
				division(t, s, "ad-2", &parent)
				persons(t, s, "p-1")
				mustDo(t, "residence", s.SaveResidence(t.Context(), &models.Residence{ID: "res-1", PersonID: "p-1", PlaceID: "ad-1"}))
			},
			referrers: []models.EntityRef{ref(models.TypeAdministrativeDivision, "ad-2"), ref(models.TypeResidence, "res-1")},
			release: func(t *testing.T, s *Store) {
				mustDo(t, "res", s.DeleteResidence(t.Context(), "res-1"))
				mustDo(t, "ad-2", s.DeleteAdministrativeDivision(t.Context(), "ad-2"))
			},
		},
		{
			name: "хранилище с источником и архивом", kind: "Repository", id: "rep-1", typ: models.TypeRepository,
			setup: func(t *testing.T, s *Store) {
				archiveChain(t, s)
				mustDo(t, "source", s.SaveSource(t.Context(), &models.Source{ID: "src-1", Kind: models.SourceKindMemory, Title: "С", RepositoryID: "rep-1"}))
			},
			referrers: []models.EntityRef{ref(models.TypeArchive, "arc-1"), ref(models.TypeSource, "src-1")},
			release: func(t *testing.T, s *Store) {
				mustDo(t, "src", s.DeleteSource(t.Context(), "src-1"))
				mustDo(t, "node", s.DeleteArchiveNode(t.Context(), "node-1"))
				mustDo(t, "arc", s.DeleteArchive(t.Context(), "arc-1"))
			},
		},
		{
			name: "источник с цитатой", kind: "Source", id: "src-1", typ: models.TypeSource,
			setup: func(t *testing.T, s *Store) {
				mustDo(t, "source", s.SaveSource(t.Context(), &models.Source{ID: "src-1", Kind: models.SourceKindMemory, Title: "С"}))
				mustDo(t, "citation", s.SaveCitation(t.Context(), &models.Citation{ID: "cit-1", SourceID: "src-1"}))
			},
			referrers: []models.EntityRef{ref(models.TypeCitation, "cit-1")},
			release:   func(t *testing.T, s *Store) { mustDo(t, "cit", s.DeleteCitation(t.Context(), "cit-1")) },
		},
		{
			name: "цитата, на которую ссылается доказательство (source_links → цель)", kind: "Citation", id: "cit-1", typ: models.TypeCitation,
			setup: func(t *testing.T, s *Store) {
				mustDo(t, "source", s.SaveSource(t.Context(), &models.Source{ID: "src-1", Kind: models.SourceKindMemory, Title: "С"}))
				mustDo(t, "citation", s.SaveCitation(t.Context(), &models.Citation{ID: "cit-1", SourceID: "src-1"}))
				mustDo(t, "person", s.SavePerson(t.Context(), &models.Person{ID: "p-1", Sources: link(models.TypePerson, "p-1")}))
			},
			referrers: []models.EntityRef{ref(models.TypePerson, "p-1")},
			release:   func(t *testing.T, s *Store) { mustDo(t, "p", s.DeletePerson(t.Context(), "p-1")) },
		},
		{
			name: "заметка с дочерней", kind: "Note", id: "note-book", typ: models.TypeNote,
			setup: func(t *testing.T, s *Store) {
				parent := models.ID("note-book")
				mustDo(t, "book", s.SaveNote(t.Context(), &models.Note{ID: "note-book", Kind: models.NoteKindBook, Title: "К"}))
				mustDo(t, "chapter", s.SaveNote(t.Context(), &models.Note{ID: "note-ch1", Kind: models.NoteKindChapter, Title: "Г", ParentID: &parent}))
			},
			referrers: []models.EntityRef{ref(models.TypeNote, "note-ch1")},
			release:   func(t *testing.T, s *Store) { mustDo(t, "ch", s.DeleteNote(t.Context(), "note-ch1")) },
		},
		{
			name: "архив с узлом", kind: "Archive", id: "arc-1", typ: models.TypeArchive,
			setup:     archiveChain,
			referrers: []models.EntityRef{ref(models.TypeArchiveNode, "node-1")},
			release:   func(t *testing.T, s *Store) { mustDo(t, "node", s.DeleteArchiveNode(t.Context(), "node-1")) },
		},
		{
			name: "узел с дочерним узлом, документом и вложением", kind: "ArchiveNode", id: "node-1", typ: models.TypeArchiveNode,
			setup: func(t *testing.T, s *Store) {
				archiveChain(t, s)
				parent := models.ID("node-1")
				mustDo(t, "child", s.SaveArchiveNode(t.Context(), &models.ArchiveNode{ID: "node-2", Type: "inventory", ArchiveID: "arc-1", ParentID: &parent}))
				mustDo(t, "doc", s.SaveArchiveDocument(t.Context(), &models.ArchiveDocument{ID: "doc-1", UnitID: "node-1", Title: "Д"}))
				mustDo(t, "att", s.SaveAttachment(t.Context(), &models.Attachment{ID: "att-1", Kind: models.AttachmentKindScan, URI: "file://1", NodeID: "node-1"}))
			},
			referrers: []models.EntityRef{
				ref(models.TypeArchiveDocument, "doc-1"), ref(models.TypeArchiveNode, "node-2"), ref(models.TypeAttachment, "att-1"),
			},
			release: func(t *testing.T, s *Store) {
				mustDo(t, "att", s.DeleteAttachment(t.Context(), "att-1"))
				mustDo(t, "doc", s.DeleteArchiveDocument(t.Context(), "doc-1"))
				mustDo(t, "child", s.DeleteArchiveNode(t.Context(), "node-2"))
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newStore(t)
			c.setup(t, s)

			before := snapshot(t, s)

			err := deleters[c.kind](s, t.Context(), c.id)

			var inUse *models.InUseError
			if !errors.As(err, &inUse) {
				t.Fatalf("Delete%s(%s) = %v, ожидался *models.InUseError", c.kind, c.id, err)
			}

			if inUse.Type != c.typ || inUse.ID != c.id || !reflect.DeepEqual(inUse.Referrers, c.referrers) {
				t.Fatalf("InUseError = %+v, ожидались тип %s, id %s, ссылающиеся %v", inUse, c.typ, c.id, c.referrers)
			}

			if !strings.Contains(err.Error(), string(c.id)) {
				t.Errorf("текст ошибки %q не называет id", err)
			}

			if got := snapshot(t, s); !reflect.DeepEqual(got, before) {
				t.Fatalf("отказ изменил таблицы:\n got  %v\n want %v", got, before)
			}

			if err := getters[c.kind](s, t.Context(), c.id); err != nil {
				t.Fatalf("сущность пропала после отказа: %v", err)
			}

			c.release(t, s)

			if err := deleters[c.kind](s, t.Context(), c.id); err != nil {
				t.Fatalf("Delete%s(%s) после снятия ссылок: %v", c.kind, c.id, err)
			}
		})
	}
}

// TestDeleteInUseCapsReferrers: список ссылающихся ограничен MaxReferrers,
// порядок стабильный (по id).
func TestDeleteInUseCapsReferrers(t *testing.T) {
	s := newStore(t)

	mustDo(t, "person", s.SavePerson(t.Context(), &models.Person{ID: "p-1"}))

	total := models.MaxReferrers + 5
	for i := 0; i < total; i++ {
		id := models.ID("ev-" + strconv.Itoa(1000+i))
		mustDo(t, "event", s.SaveEvent(t.Context(), &models.Event{
			ID: id, Type: models.EventTypeBirth, Participants: []models.EventParticipant{{PersonID: "p-1"}},
		}))
	}

	var inUse *models.InUseError
	if err := s.DeletePerson(t.Context(), "p-1"); !errors.As(err, &inUse) {
		t.Fatalf("DeletePerson = %v, ожидался *models.InUseError", err)
	}

	if len(inUse.Referrers) != models.MaxReferrers {
		t.Fatalf("ссылающихся %d, ожидалось %d", len(inUse.Referrers), models.MaxReferrers)
	}

	for i, r := range inUse.Referrers {
		want := models.EntityRef{Type: models.TypeEvent, ID: models.ID("ev-" + strconv.Itoa(1000+i))}
		if r != want {
			t.Fatalf("Referrers[%d] = %+v, ожидалось %+v", i, r, want)
		}
	}
}

// TestDeleteSetNullReferenceDoesNotBlock: вложение ссылается на документ по
// ON DELETE SET NULL — документ удаляется, а вложение остаётся без документа.
func TestDeleteSetNullReferenceDoesNotBlock(t *testing.T) {
	s := newStore(t)

	mustDo(t, "repository", s.SaveRepository(t.Context(), &models.Repository{ID: "rep-1", Name: "Р"}))
	mustDo(t, "archive", s.SaveArchive(t.Context(), &models.Archive{ID: "arc-1", Name: "А", RepositoryID: "rep-1"}))
	mustDo(t, "node", s.SaveArchiveNode(t.Context(), &models.ArchiveNode{ID: "node-1", Type: "fund", ArchiveID: "arc-1"}))
	mustDo(t, "doc", s.SaveArchiveDocument(t.Context(), &models.ArchiveDocument{ID: "doc-1", UnitID: "node-1", Title: "Д"}))

	docID := models.ID("doc-1")
	mustDo(t, "attachment", s.SaveAttachment(t.Context(), &models.Attachment{
		ID: "att-1", Kind: models.AttachmentKindScan, URI: "file://1", NodeID: "node-1", DocumentID: &docID,
	}))

	mustDo(t, "delete document", s.DeleteArchiveDocument(t.Context(), "doc-1"))

	got, err := s.GetAttachment(t.Context(), "att-1")
	mustDo(t, "get attachment", err)

	if got.DocumentID != nil {
		t.Fatalf("DocumentID = %v после удаления документа, ожидалось nil", *got.DocumentID)
	}
}

// TestDeleteDoesNotTouchOtherEntities: удаление одной сущности не задевает
// соседние строки той же таблицы.
func TestDeleteDoesNotTouchOtherEntities(t *testing.T) {
	s := newStore(t)

	for _, id := range []models.ID{"p-1", "p-2"} {
		mustDo(t, "person", s.SavePerson(t.Context(), &models.Person{
			ID: id, Names: []models.PersonName{{Surname: models.TextRef{Text: "Иванов"}, Given: models.TextRef{Text: "Пётр"}}},
			Notes: []models.TextRef{{Text: "заметка"}},
		}))
	}

	before := snapshot(t, s)

	mustDo(t, "delete", s.DeletePerson(t.Context(), "p-1"))

	got := snapshot(t, s)
	if got["persons"] != 1 || got["person_names"] != before["person_names"]/2 ||
		got["text_refs"] != before["text_refs"]/2 || got["search_index"] != before["search_index"]/2 {
		t.Fatalf("удаление p-1 задело p-2: до %v, после %v", before, got)
	}

	if _, err := s.GetPerson(t.Context(), "p-2"); err != nil {
		t.Fatalf("p-2 пропала: %v", err)
	}
}

// TestDeleteSelfReferenceDoesNotBlock: ссылка сущности на саму себя (заметка —
// собственный родитель) не считается «используется другими».
func TestDeleteSelfReferenceDoesNotBlock(t *testing.T) {
	s := newStore(t)

	mustDo(t, "note", s.SaveNote(t.Context(), &models.Note{ID: "note-1", Kind: models.NoteKindBook, Title: "К"}))

	self := models.ID("note-1")
	mustDo(t, "self parent", s.SaveNote(t.Context(), &models.Note{ID: "note-1", Kind: models.NoteKindBook, Title: "К", ParentID: &self}))
	mustDo(t, "delete", s.DeleteNote(t.Context(), "note-1"))
}

// TestDeleteInUseDeduplicatesReferrers: сущность, ссылающаяся двумя колонками
// (связь person_a = person_b), попадает в список один раз.
func TestDeleteInUseDeduplicatesReferrers(t *testing.T) {
	s := newStore(t)

	mustDo(t, "person", s.SavePerson(t.Context(), &models.Person{ID: "p-1"}))
	mustDo(t, "relation", s.SaveRelation(t.Context(), &models.Relation{
		ID: "rel-1", Kind: models.RelationKindAssociate, PersonA: "p-1", PersonB: "p-1",
	}))

	var inUse *models.InUseError
	if err := s.DeletePerson(t.Context(), "p-1"); !errors.As(err, &inUse) {
		t.Fatalf("DeletePerson = %v, ожидался *models.InUseError", err)
	}

	want := []models.EntityRef{{Type: models.TypeRelation, ID: "rel-1"}}
	if !reflect.DeepEqual(inUse.Referrers, want) {
		t.Fatalf("Referrers = %v, ожидалось %v", inUse.Referrers, want)
	}
}

// TestDeleteInUseDistinctBeforeLimit: повторные участия одной персоны в одном
// событии не съедают лимит списка — все события попадают в него по разу.
func TestDeleteInUseDistinctBeforeLimit(t *testing.T) {
	s := newStore(t)

	mustDo(t, "person", s.SavePerson(t.Context(), &models.Person{ID: "p-1"}))

	const events = 12 // по два участия в каждом: 24 строки при лимите 20

	for i := 0; i < events; i++ {
		mustDo(t, "event", s.SaveEvent(t.Context(), &models.Event{
			ID: models.ID("ev-" + strconv.Itoa(1000+i)), Type: models.EventTypeBirth,
			Participants: []models.EventParticipant{{PersonID: "p-1"}, {PersonID: "p-1", Role: "восприемник"}},
		}))
	}

	var inUse *models.InUseError
	if err := s.DeletePerson(t.Context(), "p-1"); !errors.As(err, &inUse) {
		t.Fatalf("DeletePerson = %v, ожидался *models.InUseError", err)
	}

	if len(inUse.Referrers) != events {
		t.Fatalf("ссылающихся %d, ожидалось %d (по одному на событие)", len(inUse.Referrers), events)
	}
}

// TestDeleteInUseCitationDistinctTargets: несколько ссылок на одну цель не
// съедают лимит списка и не вытесняют другие цели (source_links: цитата
// использована 25 раз одной персоной и один раз другой).
func TestDeleteInUseCitationDistinctTargets(t *testing.T) {
	s := newStore(t)

	mustDo(t, "source", s.SaveSource(t.Context(), &models.Source{ID: "src-1", Kind: models.SourceKindMemory, Title: "С"}))
	mustDo(t, "citation", s.SaveCitation(t.Context(), &models.Citation{ID: "cit-1", SourceID: "src-1"}))

	many := make([]models.SourceLink, 0, models.MaxReferrers+5)
	for i := 0; i < models.MaxReferrers+5; i++ {
		many = append(many, models.SourceLink{CitationID: "cit-1", TargetType: models.TypePerson, TargetID: "p-a",
			Role: "роль " + strconv.Itoa(i)})
	}

	mustDo(t, "person a", s.SavePerson(t.Context(), &models.Person{ID: "p-a", Sources: many}))
	mustDo(t, "person b", s.SavePerson(t.Context(), &models.Person{ID: "p-b", Sources: link(models.TypePerson, "p-b")}))

	var inUse *models.InUseError
	if err := s.DeleteCitation(t.Context(), "cit-1"); !errors.As(err, &inUse) {
		t.Fatalf("DeleteCitation = %v, ожидался *models.InUseError", err)
	}

	want := []models.EntityRef{{Type: models.TypePerson, ID: "p-a"}, {Type: models.TypePerson, ID: "p-b"}}
	if !reflect.DeepEqual(inUse.Referrers, want) {
		t.Fatalf("Referrers = %v, ожидалось %v", inUse.Referrers, want)
	}
}
