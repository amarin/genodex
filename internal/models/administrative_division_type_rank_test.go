package models

import (
	"slices"
	"testing"
)

func TestAdminDivisionTypeCanContain(t *testing.T) {
	cases := []struct {
		parent, child AdminDivisionType
		want          bool
	}{
		{AdminDivisionGuberniya, AdminDivisionUezd, true},
		{AdminDivisionUezd, AdminDivisionVolost, true},
		{AdminDivisionVolost, AdminDivisionSelo, true},
		{AdminDivisionUezd, AdminDivisionGuberniya, false},
		{AdminDivisionVolost, AdminDivisionUezd, false},
		{AdminDivisionSelo, AdminDivisionGorod, false},
		{AdminDivisionSelo, AdminDivisionVolost, false},
		{AdminDivisionSelo, AdminDivisionDerevnya, true},
		{AdminDivisionGuberniya, AdminDivisionOblast, false},
		{AdminDivisionHutor, AdminDivisionHutor, false},
		{AdminDivisionOther, AdminDivisionGuberniya, true},
		{AdminDivisionVolost, AdminDivisionOther, true},
		{AdminDivisionSelo, AdminDivisionOther, false},
		{AdminDivisionType("bogus"), AdminDivisionSelo, false},
		{AdminDivisionUezd, AdminDivisionType("bogus"), false},
	}
	for _, c := range cases {
		if got := c.parent.CanContain(c.child); got != c.want {
			t.Errorf("%s.CanContain(%s) = %v, ожидалось %v", c.parent, c.child, got, c.want)
		}
	}
}

func TestAdminDivisionTypeRanksCoverAllTypes(t *testing.T) {
	for _, typ := range AdminDivisionTypes {
		_, ok := typ.Rank()
		if ok == (typ == AdminDivisionOther) {
			t.Errorf("ранг %s: ok = %v", typ, ok)
		}
		if typ.Label() == string(typ) {
			t.Errorf("у %s нет названия", typ)
		}
	}
}

func TestAdminDivisionTypeRanksDivisionsAboveSettlements(t *testing.T) {
	for _, d := range AdminDivisionTypes {
		for _, s := range AdminDivisionTypes {
			if d == AdminDivisionOther || d.IsSettlement() || !s.IsSettlement() {
				continue
			}
			if !d.CanContain(s) {
				t.Errorf("деление %s не может содержать населённый пункт %s", d, s)
			}
		}
	}
}

func TestAdminDivisionTypeAllowedChildTypes(t *testing.T) {
	got := AdminDivisionSelo.AllowedChildTypes()
	want := []AdminDivisionType{AdminDivisionSeltso, AdminDivisionDerevnya, AdminDivisionHutor, AdminDivisionPogost}
	if !slices.Equal(got, want) {
		t.Errorf("AllowedChildTypes(selo) = %v, ожидалось %v", got, want)
	}
	if len(AdminDivisionHutor.AllowedChildTypes()) != 0 {
		t.Errorf("в хутор ничего добавить нельзя")
	}
}
