package models

import (
	"errors"
	"testing"
)

func TestPageNormalized(t *testing.T) {
	cases := []struct {
		name string
		in   Page
		want Page
	}{
		{"нулевое окно — размер по умолчанию", Page{}, Page{Limit: DefaultPageLimit}},
		{"отрицательный лимит — по умолчанию", Page{Limit: -3, Offset: 7}, Page{Limit: DefaultPageLimit, Offset: 7}},
		{"обычное окно не меняется", Page{Limit: 10, Offset: 20}, Page{Limit: 10, Offset: 20}},
		{"верхняя граница включена", Page{Limit: MaxPageLimit}, Page{Limit: MaxPageLimit}},
		{"лимит выше предела сужается", Page{Limit: MaxPageLimit + 1}, Page{Limit: MaxPageLimit}},
		{"отрицательный сдвиг — ноль", Page{Limit: 5, Offset: -1}, Page{Limit: 5}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.in.Normalized(); got != c.want {
				t.Fatalf("%+v.Normalized() = %+v, want %+v", c.in, got, c.want)
			}
		})
	}
}

func TestAccessZeroValueIsFull(t *testing.T) {
	var a Access
	if a != AccessFull {
		t.Fatalf("нулевой Access = %d, ожидался AccessFull", a)
	}

	if AccessPublic == AccessFull {
		t.Fatal("AccessPublic совпадает с AccessFull")
	}
}

func TestDivisionKindValid(t *testing.T) {
	for _, k := range []DivisionKind{"", DivisionKindSettlement} {
		if !k.Valid() {
			t.Errorf("DivisionKind(%q).Valid() = false", k)
		}
	}

	if DivisionKind("village").Valid() {
		t.Error("неизвестный вид признан допустимым")
	}
}

func TestDivisionQueryValidate(t *testing.T) {
	cases := []struct {
		name  string
		q     DivisionQuery
		field string // "" — запрос корректен
	}{
		{"пустой запрос", DivisionQuery{}, ""},
		{"вид и тип", DivisionQuery{Kind: DivisionKindSettlement, Type: AdminDivisionSelo}, ""},
		{"окно больше предела — не ошибка", DivisionQuery{Page: Page{Limit: 10 * MaxPageLimit}}, ""},
		{"неизвестный вид", DivisionQuery{Kind: "village"}, "kind"},
		{"неизвестный тип", DivisionQuery{Type: "castle"}, "type"},
		{"отрицательный размер окна", DivisionQuery{Page: Page{Limit: -1}}, "limit"},
		{"отрицательный сдвиг", DivisionQuery{Page: Page{Offset: -5}}, "offset"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.q.Validate()
			if c.field == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, ожидалось nil", err)
				}

				return
			}

			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != c.field {
				t.Fatalf("Validate() = %v, ожидалась *ValidationError по полю %q", err, c.field)
			}
		})
	}
}

func TestDivisionQueryValidateParentID(t *testing.T) {
	validAD := testID(TypeAdministrativeDivision)
	validPerson := testID(TypePerson)

	cases := []struct {
		name  string
		q     DivisionQuery
		field string // "" — запрос корректен
	}{
		{"parent_id валиден", DivisionQuery{ParentID: &validAD}, ""},
		{"parent_id неверный формат", DivisionQuery{ParentID: idPtr("nope")}, "parent_id"},
		{"parent_id не того типа", DivisionQuery{ParentID: &validPerson}, "parent_id"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.q.Validate()
			if c.field == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, ожидалось nil", err)
				}

				return
			}

			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != c.field {
				t.Fatalf("Validate() = %v, ожидалась *ValidationError по полю %q", err, c.field)
			}
		})
	}
}

func TestDivisionSearchQueryValidate(t *testing.T) {
	cases := []struct {
		name  string
		q     DivisionSearchQuery
		field string // "" — запрос корректен
	}{
		{"пустой запрос", DivisionSearchQuery{}, ""},
		{"текст с пробелами", DivisionSearchQuery{Text: " давыд "}, ""},
		{"окно больше предела — не ошибка", DivisionSearchQuery{Page: Page{Limit: 10 * MaxPageLimit}}, ""},
		{"отрицательный размер окна", DivisionSearchQuery{Page: Page{Limit: -1}}, "limit"},
		{"отрицательный сдвиг", DivisionSearchQuery{Page: Page{Offset: -5}}, "offset"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.q.Validate()
			if c.field == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, ожидалось nil", err)
				}

				return
			}

			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != c.field {
				t.Fatalf("Validate() = %v, ожидалась *ValidationError по полю %q", err, c.field)
			}
		})
	}
}

func TestDivisionQueryMatches(t *testing.T) {
	selo := AdministrativeDivision{Type: AdminDivisionSelo}
	volost := AdministrativeDivision{Type: AdminDivisionVolost}

	cases := []struct {
		name string
		q    DivisionQuery
		d    AdministrativeDivision
		want bool
	}{
		{"без фильтров", DivisionQuery{}, volost, true},
		{"вид: населённый пункт проходит", DivisionQuery{Kind: DivisionKindSettlement}, selo, true},
		{"вид: волость не проходит", DivisionQuery{Kind: DivisionKindSettlement}, volost, false},
		{"вид: единица без типа не проходит (белый список)", DivisionQuery{Kind: DivisionKindSettlement}, AdministrativeDivision{}, false},
		{"тип совпал", DivisionQuery{Type: AdminDivisionVolost}, volost, true},
		{"тип не совпал", DivisionQuery{Type: AdminDivisionVolost}, selo, false},
		{"вид и тип пересекаются", DivisionQuery{Kind: DivisionKindSettlement, Type: AdminDivisionVolost}, volost, false},
	}

	for _, c := range cases {
		if got := c.q.Matches(c.d); got != c.want {
			t.Errorf("%s: Matches = %v, ожидалось %v", c.name, got, c.want)
		}
	}
}
