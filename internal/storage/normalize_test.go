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