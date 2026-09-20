package idgen

import (
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/amarin/genodex/internal/models"
)

func TestEncode(t *testing.T) {
	tests := []struct {
		name    string
		ms      uint64
		entropy [10]byte
		want    string
	}{
		{"нули", 0, [10]byte{}, "00000000000000000000000000"},
		{"только энтропия", 0, [10]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255}, "0000000000ZZZZZZZZZZZZZZZZ"},
		{"максимум времени", 1<<48 - 1, [10]byte{}, "7ZZZZZZZZZ0000000000000000"},
		{"единица времени", 1, [10]byte{}, "00000000010000000000000000"},
		// Временна́я часть примера из спецификации ULID (01ARZ3NDEKTSV4RRFFQ69G5FAV):
		// 2016-07-30T23:54:10.259Z.
		{"пример спецификации", 1469922850259, [10]byte{}, "01ARZ3NDEK0000000000000000"},
		// Значения посчитаны независимо (Python, база 32 Crockford).
		{"2016-07-30T22:36:16.385Z", 1469918176385, [10]byte{}, "01ARYZ6S410000000000000000"},
		{"1_700_000_000_000", 1_700_000_000_000, [10]byte{}, "01HF7YAT000000000000000000"},
	}
	for _, tt := range tests {
		if got := encode(tt.ms, tt.entropy); got != tt.want {
			t.Errorf("%s: encode = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// fixedClock возвращает управляемые часы.
func fixedClock(ms *int64) func() time.Time {
	return func() time.Time { return time.UnixMilli(*ms) }
}

func TestNewValidForAllTypes(t *testing.T) {
	g := New()
	for _, typ := range models.AllTypes() {
		id := g.New(typ)
		if err := id.Validate(typ); err != nil {
			t.Errorf("%s: %v", typ, err)
		}
	}
}

func TestMonotonicWithinMillisecond(t *testing.T) {
	ms := int64(1_700_000_000_000)
	g := &Generator{now: fixedClock(&ms), rand: bytes.NewReader(make([]byte, 10))}

	prev := g.New(models.TypePerson)
	for i := 0; i < 1000; i++ {
		next := g.New(models.TypePerson)
		if next <= prev {
			t.Fatalf("id не возрастает: %q после %q", next, prev)
		}
		prev = next
	}
}

func TestClockGoingBackwardsKeepsOrder(t *testing.T) {
	ms := int64(1_700_000_000_000)
	g := &Generator{now: fixedClock(&ms), rand: bytes.NewReader(make([]byte, 10))}

	first := g.New(models.TypePerson)
	ms -= 5000 // часы ушли назад
	second := g.New(models.TypePerson)
	if second <= first {
		t.Errorf("после отката часов %q не больше %q", second, first)
	}
}

func TestNewMillisecondAdvances(t *testing.T) {
	ms := int64(1_700_000_000_000)
	// Две порции энтропии: вторая нужна при переходе на новую миллисекунду.
	src := append(bytes.Repeat([]byte{0}, 10), bytes.Repeat([]byte{0}, 10)...)
	g := &Generator{now: fixedClock(&ms), rand: bytes.NewReader(src)}

	a := g.New(models.TypePerson)
	ms++
	b := g.New(models.TypePerson)
	if b <= a {
		t.Errorf("%q не больше %q после смены миллисекунды", b, a)
	}
}

// Переполнение 80 бит энтропии в одну миллисекунду переносится во время.
func TestEntropyOverflowMovesToNextMillisecond(t *testing.T) {
	ms := int64(1_700_000_000_000)
	src := append(bytes.Repeat([]byte{0xFF}, 10), make([]byte, 10)...)
	g := &Generator{now: fixedClock(&ms), rand: bytes.NewReader(src)}

	first := g.New(models.TypePerson)
	second := g.New(models.TypePerson) // та же миллисекунда, энтропия переполнена
	if second <= first {
		t.Errorf("%q не больше %q после переполнения энтропии", second, first)
	}
	if err := second.Validate(models.TypePerson); err != nil {
		t.Errorf("после переполнения id невалиден: %v", err)
	}
}

func TestRealClockStrictlyIncreasingAndUnique(t *testing.T) {
	g := New()
	seen := make(map[models.ID]struct{}, 10000)
	var prev models.ID
	for i := 0; i < 10000; i++ {
		id := g.New(models.TypeEvent)
		if _, dup := seen[id]; dup {
			t.Fatalf("повтор id %q", id)
		}
		seen[id] = struct{}{}
		if id <= prev {
			t.Fatalf("id не возрастает: %q после %q", id, prev)
		}
		prev = id
	}
}

func TestConcurrentUnique(t *testing.T) {
	g := New()
	const workers, per = 8, 1000

	var (
		mu   sync.Mutex
		seen = make(map[models.ID]struct{}, workers*per)
		wg   sync.WaitGroup
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := make([]models.ID, 0, per)
			for i := 0; i < per; i++ {
				local = append(local, g.New(models.TypeSource))
			}
			mu.Lock()
			for _, id := range local {
				if _, dup := seen[id]; dup {
					t.Errorf("повтор id %q", id)
				}
				seen[id] = struct{}{}
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(seen) != workers*per {
		t.Errorf("уникальных id %d, want %d", len(seen), workers*per)
	}
}

func TestUnknownTypePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("New для типа без префикса должен паниковать")
		}
	}()
	New().New(models.Type("nonsense"))
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("нет энтропии") }

func TestRandomSourceFailurePanics(t *testing.T) {
	ms := int64(1_700_000_000_000)
	g := &Generator{now: fixedClock(&ms), rand: errReader{}}

	defer func() {
		if recover() == nil {
			t.Error("сбой источника случайности должен паниковать")
		}
	}()
	g.New(models.TypePerson)
}
