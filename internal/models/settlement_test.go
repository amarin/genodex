package models

import "testing"

// Тест фиксирует решение #17: населённый пункт — вид AdministrativeDivision.type.
// IsSettlement — белый список: только семь видов населённых пунктов.
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
		{AdminDivisionGovernorate, false},
		{AdminDivisionDistrict, false},
		{AdminDivisionVolost, false},
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
