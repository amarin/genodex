package models

import "testing"

// Тест фиксирует решение #17: населённый пункт — вид AdministrativeDivision.type.
func TestSettlementIsAdminDivisionKind(t *testing.T) {
	selo := AdminDivisionType("selo")
	if !selo.IsSettlement() {
		t.Fatal("selo должен быть населённым пунктом")
	}
	volost := AdminDivisionType("volost")
	if volost.IsSettlement() {
		t.Fatal("volost не населённый пункт")
	}
}
