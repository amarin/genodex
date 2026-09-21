package sqlstore

import (
	"reflect"
	"testing"
)

func TestPlaceholders(t *testing.T) {
	if got := placeholders(3); got != "?, ?, ?" {
		t.Fatalf("placeholders(3) = %q", got)
	}

	if got := placeholders(1); got != "?" {
		t.Fatalf("placeholders(1) = %q", got)
	}
}

func TestForChunksSplitsAtInChunk(t *testing.T) {
	vals := make([]int, 2*inChunk+200)
	for i := range vals {
		vals[i] = i
	}

	var sizes []int

	seen := 0

	if err := forChunks(vals, func(chunk []int) error {
		sizes = append(sizes, len(chunk))

		for _, v := range chunk {
			if v != seen {
				t.Fatalf("значение %d не на своём месте (ожидалось %d)", v, seen)
			}

			seen++
		}

		return nil
	}); err != nil {
		t.Fatalf("forChunks: %v", err)
	}

	if want := []int{inChunk, inChunk, 200}; !reflect.DeepEqual(sizes, want) {
		t.Fatalf("размеры кусков %v, ожидалось %v", sizes, want)
	}

	if seen != len(vals) {
		t.Fatalf("обработано %d значений из %d", seen, len(vals))
	}

	calls := 0
	_ = forChunks([]int(nil), func([]int) error { calls++; return nil })

	if calls != 0 {
		t.Fatalf("пустой список вызвал fn %d раз", calls)
	}
}

func TestNonZero(t *testing.T) {
	got := nonZero([]int64{0, 5, 0, 7})
	if want := []int64{5, 7}; !reflect.DeepEqual(got, want) {
		t.Fatalf("nonZero = %v, ожидалось %v", got, want)
	}
}
