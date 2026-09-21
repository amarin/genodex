package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// countingExec считает запросы, проходящие через исполнитель Store.
type countingExec struct {
	x sqlExecutor
	n atomic.Int64
}

func (c *countingExec) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	c.n.Add(1)

	return c.x.ExecContext(ctx, q, args...)
}

func (c *countingExec) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	c.n.Add(1)

	return c.x.QueryContext(ctx, q, args...)
}

func (c *countingExec) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	c.n.Add(1)

	return c.x.QueryRowContext(ctx, q, args...)
}

// counted возвращает копию Store, считающую запросы, и счётчик. Кэш графа схемы
// прогревается заранее, чтобы его загрузка не попала в подсчёт.
func counted(t *testing.T, s *Store) (*Store, *countingExec) {
	t.Helper()

	if _, err := s.graph(t.Context()); err != nil {
		t.Fatalf("graph: %v", err)
	}

	c := &countingExec{x: s.exec}

	return &Store{st: s.st, db: s.db, exec: c, cache: s.cache}, c
}

// batchDate — дата для фикстур: разные i дают разные значения; чётные — с
// верхней границей диапазона (between), нечётные кратные трём — по юлианскому
// календарю, остальные — точная григорианская дата.
func batchDate(i int) *models.FactDate {
	d := &models.FactDate{
		Year: 1800 + i%100, Month: 1 + i%12, Day: 1 + i%28,
		Precision: models.PrecisionDay, Modifier: models.ModifierExact,
	}

	if i%2 == 0 {
		d.Modifier = models.ModifierBetween
		d.YearTo, d.MonthTo, d.DayTo = d.Year+1, 1+(i+3)%12, 1+(i+5)%28
	}

	if i%3 == 0 {
		d.Calendar = models.FactCalendarJulian
	}

	return d
}

// batchPerson строит персону: full — все поля заполнены; иначе по i получается
// минимальная, с одним именем без дат или без имён (нули и пустые списки —
// отдельные случаи пакетной сборки).
func batchPerson(i int, full bool) *models.Person {
	id := models.ID(fmt.Sprintf("p-%04d", i))
	p := &models.Person{ID: id, Gender: models.PersonGenderMale, Private: i%4 == 0}

	switch {
	case full || i%3 == 0:
		p.Names = []models.PersonName{
			{Type: models.PersonNameMain, Surname: models.TextRef{Text: "Иванов", Ref: "sur-1", Type: models.TypeSurname},
				Given: models.TextRef{Text: "Пётр"}, Patronymic: models.TextRef{Text: "Сергеевич"},
				Prefix: "фон", Suffix: "ст.", Since: batchDate(i), Until: batchDate(i + 1)},
			{Type: models.PersonNameBirth, Surname: models.TextRef{Text: "Петров"}, Given: models.TextRef{Text: "Пётр"}},
		}
		p.Estates = []models.TextRef{{Text: "крестьянин"}, {Text: "мещанин"}}
		p.Titles = []models.TextRef{{Text: "унтер-офицер"}}
		p.Nicknames = []models.TextRef{{Text: "Петруха"}}
		p.Notes = []models.TextRef{{Text: "из ревизии"}, {Text: "запись " + string(id)}}
		p.Sources = link(models.TypePerson, id)
	case i%3 == 2:
		p.Names = []models.PersonName{{
			Surname: models.TextRef{Text: "Сидоров"}, Given: models.TextRef{Text: "Иван"},
			Since: batchDate(i), // только начало периода: Until остаётся nil
		}}
		p.Notes = []models.TextRef{{Text: "только заметка"}}
	}

	return p
}

// batchDivision строит единицу деления (см. batchPerson).
func batchDivision(i int, full bool, parent *models.ID) *models.AdministrativeDivision {
	a := &models.AdministrativeDivision{
		ID: models.ID(fmt.Sprintf("ad-%04d", i)), Name: fmt.Sprintf("Деревня %d", i),
		Type: models.AdminDivisionDerevnya, ParentID: parent,
	}

	if full || i%3 == 0 {
		a.Items = []models.TextRef{{Text: "двор Ивановых"}, {Text: "двор Петровых"}}
		a.Variants = []string{"Давыдовка", "Давыдово-Никольское"}
		a.Renames = []models.NamedPeriod{{Text: "Давыдовка", Since: "1800", Until: "1850"}, {Text: "Давыдово", Since: "1850"}}
		a.Successors = []models.TextRef{{Text: "Давыдово (совр.)"}}
		a.Since, a.Until = batchDate(i), batchDate(i+5)
		a.Notes = []models.TextRef{{Text: "упомянуто"}}
		a.Sources = link(models.TypeAdministrativeDivision, a.ID)
	} else if i%3 == 2 {
		a.Notes = []models.TextRef{{Text: "только заметка"}}
	}

	return a
}

// seedBatch сохраняет n персон и n единиц деления (одной транзакцией).
func seedBatch(t *testing.T, s *Store, n int, full bool) {
	t.Helper()

	ctx := t.Context()
	seedSource(t, s)
	mustDo(t, "citation", s.SaveCitation(ctx, &models.Citation{ID: "cit-1", SourceID: "src-1"}))

	mustDo(t, "seed", s.InTx(ctx, func(tx store.Store) error {
		root := models.ID("ad-root")
		if err := tx.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
			ID: root, Name: "Московская", Type: models.AdminDivisionGovernorate,
		}); err != nil {
			return err
		}

		for i := 0; i < n; i++ {
			if err := tx.SavePerson(ctx, batchPerson(i, full)); err != nil {
				return err
			}

			if err := tx.SaveAdministrativeDivision(ctx, batchDivision(i, full, &root)); err != nil {
				return err
			}
		}

		return nil
	}))
}

// TestBatchListMatchesPerItemLoad: пакетный список совпадает с прежней
// поштучной загрузкой каждой сущности (включая nil против пустых срезов) при
// обоих режимах доступа.
func TestBatchListMatchesPerItemLoad(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()

	seedBatch(t, s, 40, false)

	for _, access := range []models.Access{models.AccessFull, models.AccessPublic} {
		people, err := s.ListPeople(ctx, access, models.Page{})
		mustDo(t, "list people", err)

		if len(people) == 0 {
			t.Fatal("ListPeople вернул пустой список")
		}

		for _, p := range people {
			want, err := legacyGetPerson(s, ctx, p.ID)
			mustDo(t, "legacy person "+string(p.ID), err)

			if !reflect.DeepEqual(p, want) {
				t.Fatalf("персона %s\n пакетно  %+v\n поштучно %+v", p.ID, p, want)
			}
		}

		divisions, err := s.ListAdministrativeDivisions(ctx, access, models.Page{Limit: 100})
		mustDo(t, "list divisions", err)

		for _, d := range divisions {
			want, err := legacyGetDivision(s, ctx, d.ID)
			mustDo(t, "legacy division "+string(d.ID), err)

			if !reflect.DeepEqual(d, want) {
				t.Fatalf("деление %s\n пакетно  %+v\n поштучно %+v", d.ID, d, want)
			}
		}
	}

	// приватные персоны (i%4 == 0) скрыты в публичном режиме, деления — нет
	public, err := s.ListPeople(ctx, models.AccessPublic, models.Page{Limit: 100})
	mustDo(t, "public people", err)

	if len(public) != 30 {
		t.Fatalf("публичных персон %d, ожидалось 30", len(public))
	}
}

// TestBatchListAcrossChunkBoundaries: окно из 500 персон разбивает IN-списки
// значений на несколько кусков (3000 id text_refs — 6 кусков, 1000 id dates —
// 2 куска; списки по владельцам ровно на одном куске в 500), и ни одно
// значение не теряется. Хвост за пределами первого окна тоже сверяется с эталоном.
func TestBatchListAcrossChunkBoundaries(t *testing.T) {
	s := newStore(t)
	ctx := t.Context() // тысячи эталонных запросов: без короткого таймаута

	seedBatch(t, s, inChunk+20, true)

	first, err := s.ListPeople(ctx, models.AccessFull, models.Page{Limit: models.MaxPageLimit})
	mustDo(t, "first window", err)

	if len(first) != models.MaxPageLimit {
		t.Fatalf("первое окно %d персон, ожидалось %d", len(first), models.MaxPageLimit)
	}

	for _, p := range first {
		want, err := legacyGetPerson(s, ctx, p.ID)
		mustDo(t, "legacy "+string(p.ID), err)

		if !reflect.DeepEqual(p, want) {
			t.Fatalf("персона %s расходится с поштучной загрузкой:\n пакетно  %+v\n поштучно %+v", p.ID, p, want)
		}
	}

	tail, err := s.ListPeople(ctx, models.AccessFull, models.Page{Limit: models.MaxPageLimit, Offset: models.MaxPageLimit})
	mustDo(t, "tail", err)

	if len(tail) != 20 {
		t.Fatalf("хвост %d персон, ожидалось 20", len(tail))
	}

	for _, p := range tail {
		want, err := legacyGetPerson(s, ctx, p.ID)
		mustDo(t, "legacy tail "+string(p.ID), err)

		if !reflect.DeepEqual(p, want) {
			t.Fatalf("персона %s из хвоста расходится с поштучной загрузкой:\n пакетно  %+v\n поштучно %+v", p.ID, p, want)
		}
	}

	// деления: корень + 520 — второе окно тоже сверяется с эталоном
	for _, page := range []models.Page{
		{Limit: models.MaxPageLimit},
		{Limit: models.MaxPageLimit, Offset: models.MaxPageLimit},
	} {
		divisions, err := s.ListAdministrativeDivisions(ctx, models.AccessFull, page)
		mustDo(t, "divisions", err)

		if len(divisions) == 0 {
			t.Fatalf("окно делений %+v пусто", page)
		}

		for _, d := range divisions {
			want, err := legacyGetDivision(s, ctx, d.ID)
			mustDo(t, "legacy "+string(d.ID), err)

			if !reflect.DeepEqual(d, want) {
				t.Fatalf("деление %s расходится с поштучной загрузкой:\n пакетно  %+v\n поштучно %+v", d.ID, d, want)
			}
		}
	}
}

// TestListQueryCountDoesNotDependOnRowCount: число SQL-запросов ListPeople и
// ListAdministrativeDivisions одно и то же для 3 и 30 полностью заполненных
// строк (в пределах одного куска IN) и равно известной константе.
func TestListQueryCountDoesNotDependOnRowCount(t *testing.T) {
	const (
		// Значения верны для полностью заполненных строк (full = true): загрузка
		// text_refs/dates не делает запроса на пустой список id.
		// id окна + persons, person_names, text_refs, dates, 4 списка, source_links
		peopleQueries = 10
		// id окна + деления, dates, 3 списка, variants, renames, source_links
		divisionQueries = 9
		// то же без запроса id окна
		getPersonQueries = peopleQueries - 1
	)

	measure := func(n int) (people, divisions int64) {
		s := newStore(t)
		seedBatch(t, s, n, true)

		c, counter := counted(t, s)

		got, err := c.ListPeople(t.Context(), models.AccessFull, models.Page{Limit: models.MaxPageLimit})
		mustDo(t, "list people", err)

		if len(got) != n {
			t.Fatalf("ListPeople вернул %d персон, ожидалось %d", len(got), n)
		}

		people = counter.n.Swap(0)

		gotDivs, err := c.ListAdministrativeDivisions(t.Context(), models.AccessFull, models.Page{Limit: models.MaxPageLimit})
		mustDo(t, "list divisions", err)

		if len(gotDivs) != n+1 { // и корень
			t.Fatalf("ListAdministrativeDivisions вернул %d, ожидалось %d", len(gotDivs), n+1)
		}

		return people, counter.n.Swap(0)
	}

	smallPeople, smallDivs := measure(3)
	bigPeople, bigDivs := measure(30)

	if smallPeople != bigPeople || smallDivs != bigDivs {
		t.Fatalf("число запросов зависит от числа строк: персоны %d→%d, деления %d→%d",
			smallPeople, bigPeople, smallDivs, bigDivs)
	}

	if bigPeople != peopleQueries || bigDivs != divisionQueries {
		t.Fatalf("число запросов изменилось: персоны %d (было %d), деления %d (было %d); "+
			"если набор запросов изменён осознанно — поправьте константы", bigPeople, peopleQueries, bigDivs, divisionQueries)
	}

	// одиночное чтение идёт тем же пакетным путём
	s := newStore(t)
	seedBatch(t, s, 3, true)

	c, counter := counted(t, s)
	if _, err := c.GetPerson(t.Context(), "p-0001"); err != nil {
		t.Fatalf("GetPerson: %v", err)
	}

	if got := counter.n.Load(); got != getPersonQueries {
		t.Fatalf("GetPerson выполнил %d запросов, ожидалось %d", got, getPersonQueries)
	}
}
