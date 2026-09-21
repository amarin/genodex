package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// withTimeout ограничивает тест: зависание на единственном соединении должно
// стать ошибкой контекста, а не бесконечным ожиданием.
func withTimeout(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	return ctx
}

// seedSource сохраняет источник — цель для цитаты.
func seedSource(t *testing.T, s *Store) {
	t.Helper()

	mustDo(t, "source", s.SaveSource(t.Context(), &models.Source{
		ID: "src-1", Kind: models.SourceKindMemory, Title: "Рассказ",
	}))
}

var errBoom = errors.New("boom")

// TestInTxCommitsCitationAndPersonAtomically: цитата и ссылающаяся на неё
// персона пишутся одной транзакцией и после успеха видны обе.
func TestInTxCommitsCitationAndPersonAtomically(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)
	seedSource(t, s)

	err := s.InTx(ctx, func(tx store.Store) error {
		if err := tx.SaveCitation(ctx, &models.Citation{ID: "cit-1", SourceID: "src-1"}); err != nil {
			return err
		}

		return tx.SavePerson(ctx, &models.Person{ID: "p-1", Sources: link(models.TypePerson, "p-1")})
	})
	mustDo(t, "InTx", err)

	if _, err := s.GetCitation(ctx, "cit-1"); err != nil {
		t.Fatalf("цитата не сохранена: %v", err)
	}

	got, err := s.GetPerson(ctx, "p-1")
	mustDo(t, "get person", err)

	if len(got.Sources) != 1 {
		t.Fatalf("у персоны %d доказательств, ожидалось 1", len(got.Sources))
	}
}

// TestInTxRollsBackEverythingOnError: ошибка после записи обеих сущностей
// откатывает обе и возвращается как есть.
func TestInTxRollsBackEverythingOnError(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)
	seedSource(t, s)

	before := snapshot(t, s)

	err := s.InTx(ctx, func(tx store.Store) error {
		mustDo(t, "citation", tx.SaveCitation(ctx, &models.Citation{ID: "cit-1", SourceID: "src-1"}))
		mustDo(t, "person", tx.SavePerson(ctx, &models.Person{ID: "p-1", Sources: link(models.TypePerson, "p-1")}))

		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("InTx = %v, ожидалась errBoom", err)
	}

	for _, get := range []func() error{
		func() error { _, err := s.GetCitation(ctx, "cit-1"); return err },
		func() error { _, err := s.GetPerson(ctx, "p-1"); return err },
	} {
		if err := get(); !errors.Is(err, models.ErrNotFound) {
			t.Fatalf("после отката Get = %v, ожидалось ErrNotFound", err)
		}
	}

	if got := snapshot(t, s); !reflect.DeepEqual(got, before) {
		t.Fatalf("откат оставил следы:\n got  %v\n want %v", got, before)
	}
}

// TestInTxSeesOwnWrites: внутри транзакции читаются её же незафиксированные
// записи, включая удаление (граф схемы на холодном Store грузится в той же
// транзакции, а не блокируется на соединении).
func TestInTxSeesOwnWrites(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	mustDo(t, "seed", s.SavePerson(ctx, &models.Person{ID: "p-old"}))

	err := s.InTx(ctx, func(tx store.Store) error {
		mustDo(t, "save", tx.SavePerson(ctx, &models.Person{ID: "p-1"}))

		if _, err := tx.GetPerson(ctx, "p-1"); err != nil {
			t.Errorf("запись не видна внутри транзакции: %v", err)
		}

		if list, err := tx.ListPeople(ctx); err != nil || len(list) != 2 {
			t.Errorf("ListPeople внутри транзакции: len=%d err=%v, ожидалось 2", len(list), err)
		}

		mustDo(t, "delete", tx.DeletePerson(ctx, "p-old"))

		if _, err := tx.GetPerson(ctx, "p-old"); !errors.Is(err, models.ErrNotFound) {
			t.Errorf("удалённая внутри транзакции персона: %v, ожидалось ErrNotFound", err)
		}

		return nil
	})
	mustDo(t, "InTx", err)

	if _, err := s.GetPerson(ctx, "p-old"); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("удаление не зафиксировано: %v", err)
	}
}

// TestInTxDeleteInUseRollsBackNeighbours: *InUseError из Delete* внутри
// транзакции доходит до вызывающего, соседние записи откатываются.
func TestInTxDeleteInUseRollsBackNeighbours(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	mustDo(t, "p-1", s.SavePerson(ctx, &models.Person{ID: "p-1"}))
	mustDo(t, "p-2", s.SavePerson(ctx, &models.Person{ID: "p-2"}))
	mustDo(t, "relation", s.SaveRelation(ctx, &models.Relation{
		ID: "rel-1", Kind: models.RelationKindMarriage, PersonA: "p-1", PersonB: "p-2",
	}))

	err := s.InTx(ctx, func(tx store.Store) error {
		mustDo(t, "neighbour", tx.SavePerson(ctx, &models.Person{ID: "p-3"}))

		return tx.DeletePerson(ctx, "p-1")
	})

	var inUse *models.InUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("InTx = %v, ожидался *models.InUseError", err)
	}

	if _, err := s.GetPerson(ctx, "p-3"); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("соседняя запись не откатилась: %v", err)
	}
}

// TestInTxNestedUsesSameTransaction: вложенный InTx работает в той же
// транзакции — видит незафиксированные записи внешнего, а откат внешнего
// откатывает и записи вложенного.
func TestInTxNestedUsesSameTransaction(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	err := s.InTx(ctx, func(outer store.Store) error {
		mustDo(t, "outer save", outer.SavePerson(ctx, &models.Person{ID: "p-1"}))

		mustDo(t, "inner", outer.InTx(ctx, func(inner store.Store) error {
			if _, err := inner.GetPerson(ctx, "p-1"); err != nil {
				t.Errorf("вложенный вызов не видит запись внешнего: %v", err)
			}

			return inner.SavePerson(ctx, &models.Person{ID: "p-2"})
		}))

		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("InTx = %v, ожидалась errBoom", err)
	}

	for _, id := range []models.ID{"p-1", "p-2"} {
		if _, err := s.GetPerson(ctx, id); !errors.Is(err, models.ErrNotFound) {
			t.Fatalf("после отката внешнего %s: %v, ожидалось ErrNotFound", id, err)
		}
	}
}

// TestInTxNestedErrorPropagatesToOuter: ошибка вложенного вызова возвращается
// внешней функции.
func TestInTxNestedErrorPropagatesToOuter(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	err := s.InTx(ctx, func(outer store.Store) error {
		return outer.InTx(ctx, func(store.Store) error { return errBoom })
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("InTx = %v, ожидалась errBoom", err)
	}
}

// TestInTxNestedHasNoSavepoint фиксирует документированное ограничение:
// вложенный вызов не откатывается отдельно, поэтому ошибка, проглоченная
// внешней функцией, оставляет записи вложенного в общей транзакции.
func TestInTxNestedHasNoSavepoint(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	err := s.InTx(ctx, func(outer store.Store) error {
		_ = outer.InTx(ctx, func(inner store.Store) error {
			mustDo(t, "inner save", inner.SavePerson(ctx, &models.Person{ID: "p-2"}))

			return errBoom
		})

		return nil
	})
	mustDo(t, "InTx", err)

	if _, err := s.GetPerson(ctx, "p-2"); err != nil {
		t.Fatalf("запись вложенного вызова пропала: %v", err)
	}
}

// TestInTxPanicRollsBackAndKeepsStoreUsable: паника в fn откатывает
// транзакцию и не оставляет соединение занятым.
func TestInTxPanicRollsBackAndKeepsStoreUsable(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	func() {
		defer func() {
			if p := recover(); p != "boom" {
				t.Fatalf("паника = %v, ожидалась boom", p)
			}
		}()

		_ = s.InTx(ctx, func(tx store.Store) error {
			mustDo(t, "save", tx.SavePerson(ctx, &models.Person{ID: "p-1"}))
			panic("boom")
		})
	}()

	if _, err := s.GetPerson(ctx, "p-1"); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("после паники Get = %v, ожидалось ErrNotFound (транзакция откатана, соединение свободно)", err)
	}

	mustDo(t, "save after panic", s.SavePerson(ctx, &models.Person{ID: "p-2"}))
}

// TestInTxCanceledContext: отменённый контекст — ошибка контекста, fn не
// вызывается.
func TestInTxCanceledContext(t *testing.T) {
	s := newStore(t)

	err := s.InTx(canceled(t), func(store.Store) error {
		t.Error("fn не должна вызываться при отменённом контексте")

		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("InTx = %v, ожидалось context.Canceled", err)
	}
}

// TestInTxEscapedStoreFailsAfterCommit: Store, сохранённый за пределами
// InTx, после фиксации возвращает ошибку завершённой транзакции, а не виснет.
func TestInTxEscapedStoreFailsAfterCommit(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	var leaked store.Store

	mustDo(t, "InTx", s.InTx(ctx, func(tx store.Store) error {
		leaked = tx

		return nil
	}))

	if _, err := leaked.GetPerson(ctx, "p-1"); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("Get на просочившемся Store = %v, ожидалось sql.ErrTxDone", err)
	}
}

// TestInTxColdGraphDoesNotDeadlockWithOutsideDelete: на холодном кэше графа
// схемы Delete вне транзакции ждёт единственное соединение, которое держит
// InTx; Delete внутри InTx не должен ждать замок кэша, занятый первым.
func TestInTxColdGraphDoesNotDeadlockWithOutsideDelete(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	mustDo(t, "p-a", s.SavePerson(ctx, &models.Person{ID: "p-a"}))
	mustDo(t, "p-b", s.SavePerson(ctx, &models.Person{ID: "p-b"}))

	entered := make(chan struct{})
	outside := make(chan error, 1)
	inside := make(chan error, 1)

	go func() {
		inside <- s.InTx(ctx, func(tx store.Store) error {
			close(entered)

			// дать внешнему Delete войти в загрузку графа и встать в очередь
			// за соединением
			time.Sleep(300 * time.Millisecond)

			return tx.DeletePerson(ctx, "p-b")
		})
	}()

	<-entered

	go func() { outside <- s.DeletePerson(ctx, "p-a") }()

	for name, ch := range map[string]chan error{"внутри InTx": inside, "снаружи": outside} {
		select {
		case err := <-ch:
			mustDo(t, "Delete "+name, err)
		case <-ctx.Done():
			t.Fatalf("Delete %s завис: взаимоблокировка замка кэша графа и соединения", name)
		}
	}
}

// TestInTxNestedCanceledContext: вложенный InTx с отменённым контекстом —
// ошибка контекста, вложенная fn не вызывается.
func TestInTxNestedCanceledContext(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	err := s.InTx(ctx, func(outer store.Store) error {
		return outer.InTx(canceled(t), func(store.Store) error {
			t.Error("вложенная fn не должна вызываться при отменённом контексте")

			return nil
		})
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("InTx = %v, ожидалось context.Canceled", err)
	}
}
