package auth

import "testing"

// TestEncodeULIDMatchesKnownVectors: те же контрольные значения, что
// internal/idgen/idgen_test.go:TestEncode (тот же алгоритм).
func TestEncodeULIDMatchesKnownVectors(t *testing.T) {
	tests := []struct {
		name    string
		ms      uint64
		entropy [10]byte
		want    string
	}{
		{"нули", 0, [10]byte{}, "00000000000000000000000000"},
		{"только энтропия", 0, [10]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255}, "0000000000ZZZZZZZZZZZZZZZZ"},
		{"максимум времени", 1<<48 - 1, [10]byte{}, "7ZZZZZZZZZ0000000000000000"},
		{"пример спецификации", 1469922850259, [10]byte{}, "01ARZ3NDEK0000000000000000"},
	}
	for _, tt := range tests {
		if got := encodeULID(tt.ms, tt.entropy); got != tt.want {
			t.Errorf("%s: encodeULID = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestNewIDValidForAllKinds(t *testing.T) {
	for _, k := range []Kind{KindOwner, KindSession, KindAPIToken, KindInvite} {
		id, err := newID(k)
		if err != nil {
			t.Fatalf("%s: %v", k, err)
		}
		if err := id.Validate(k); err != nil {
			t.Errorf("%s: %v", k, err)
		}
	}
}

func TestNewIDUnknownKindErrors(t *testing.T) {
	if _, err := newID(Kind("nonsense")); err == nil {
		t.Fatal("ожидалась ошибка для неизвестного вида")
	}
}

// TestNewIDUniqueOnSeries: 10 000 значений одного вида без повторов.
func TestNewIDUniqueOnSeries(t *testing.T) {
	seen := make(map[ID]struct{}, 10000)
	for i := 0; i < 10000; i++ {
		id, err := newID(KindSession)
		if err != nil {
			t.Fatalf("newID: %v", err)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("повтор id %q", id)
		}
		seen[id] = struct{}{}
	}
}
