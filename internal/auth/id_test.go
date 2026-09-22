package auth

import (
	"errors"
	"testing"
)

func TestBuildIDAndValidate(t *testing.T) {
	id, err := buildID(KindOwner, "01ARZ3NDEKTSV4RRFFQ69G5FA9")
	if err != nil {
		t.Fatalf("buildID: %v", err)
	}
	if id != "OW-01ARZ3NDEKTSV4RRFFQ69G5FA9" {
		t.Fatalf("id = %q", id)
	}
	if err := id.Validate(KindOwner); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if err := id.Validate(KindSession); err == nil {
		t.Fatal("ожидалась ошибка при несовпадении вида")
	}
}

func TestBuildIDUnknownKind(t *testing.T) {
	if _, err := buildID(Kind("nonsense"), "01ARZ3NDEKTSV4RRFFQ69G5FA9"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("err = %v, ожидался ErrInvalidID", err)
	}
}

func TestValidateRejectsBadFormat(t *testing.T) {
	for _, id := range []ID{
		"", "OW", "OW-", "OW-tooshort", "XX-01ARZ3NDEKTSV4RRFFQ69G5FA9",
		"OW-01ARZ3NDEKTSV4RRFFQ69G5FAI", // I — вне алфавита Crockford
		"OW-81ARZ3NDEKTSV4RRFFQ69G5FA9", // первый символ > 7
	} {
		if err := id.Validate(KindOwner); !errors.Is(err, ErrInvalidID) {
			t.Errorf("%q: err = %v, ожидался ErrInvalidID", id, err)
		}
	}
}

func TestAllKindsHavePrefix(t *testing.T) {
	for _, k := range []Kind{KindOwner, KindSession, KindAPIToken, KindInvite} {
		if kindPrefix[k] == "" {
			t.Errorf("у вида %q нет префикса", k)
		}
	}
}

func TestKindPrefixesAreUnique(t *testing.T) {
	seen := map[string]Kind{}
	for k, p := range kindPrefix {
		if other, dup := seen[p]; dup {
			t.Errorf("префикс %q у видов %q и %q", p, other, k)
		}
		seen[p] = k
	}
}
