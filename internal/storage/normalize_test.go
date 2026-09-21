package storage

import "testing"

func TestNormalizeСемёновVsСеменов(t *testing.T) {
	a := Normalize("Семёнов")
	b := Normalize("Семенов")
	if a != b {
		t.Fatalf("ё должен приравниваться к е: %q != %q", a, b)
	}
}

func TestNormalizeLowercase(t *testing.T) {
	if got := Normalize("СЕМЕНОВ"); got != Normalize("семенов") {
		t.Fatalf("Normalize(uppercase) = %q", got)
	}
}

func TestNormalizeTrimsEdgeSpaces(t *testing.T) {
	if got := Normalize("  Давыдово \t"); got != "давыдово" {
		t.Fatalf("Normalize с пробелами по краям = %q, want %q", got, "давыдово")
	}

	if got := Normalize("Иван Иванов"); got != "иван иванов" {
		t.Fatalf("Normalize не должен трогать внутренние пробелы: %q", got)
	}
}
