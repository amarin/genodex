package models

import "testing"

// Тест фиксирует решение #17: населённый пункт — вид AdministrativeDivision.type.
// IsSettlement — белый список: только виды населённых пунктов.
func TestSettlementIsAdminDivisionKind(t *testing.T) {
	cases := []struct {
		typ  AdminDivisionType
		want bool
	}{
		{AdminDivisionGorod, true},
		{AdminDivisionSelo, true},
		{AdminDivisionDerevnya, true},
		{AdminDivisionHutor, true},
		{AdminDivisionPogost, true},
		{AdminDivisionStanitsa, true},
		{AdminDivisionMestechko, true},
		{AdminDivisionPoselok, true},
		{AdminDivisionSloboda, true},
		{AdminDivisionSeltso, true},
		{AdminDivisionGuberniya, false},
		{AdminDivisionUezd, false},
		{AdminDivisionVolost, false},
		{AdminDivisionNamestnichestvo, false},
		{AdminDivisionProvintsiya, false},
		{AdminDivisionStan, false},
		{AdminDivisionOblast, false},
		{AdminDivisionOkrug, false},
		{AdminDivisionRespublika, false},
		{AdminDivisionKrai, false},
		{AdminDivisionRayon, false},
		{AdminDivisionSelsovet, false},
		{"governorate", false},
		{"district", false},
		{AdminDivisionOther, false},
		{"", false},
		{"губерния", false},
		{"selo ", false},
	}
	for _, c := range cases {
		if got := c.typ.IsSettlement(); got != c.want {
			t.Errorf("%q.IsSettlement() = %v, want %v", c.typ, got, c.want)
		}
	}
}

// Прежние многозначные коды (governorate — губерния/область/край, district —
// уезд/округ/район) больше не допустимы: каждый тип однозначно называет термин.
func TestAdminDivisionTypeLegacyCodesInvalid(t *testing.T) {
	for _, legacy := range []AdminDivisionType{"governorate", "district"} {
		if legacy.Valid() {
			t.Errorf("%q.Valid() = true, want false", legacy)
		}
	}
}

func TestAdminDivisionTypesUnique(t *testing.T) {
	seen := map[AdminDivisionType]bool{}
	for _, typ := range AdminDivisionTypes {
		if seen[typ] {
			t.Errorf("тип %q повторяется в AdminDivisionTypes", typ)
		}
		seen[typ] = true
	}
}
