package models

import "testing"

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
