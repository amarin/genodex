package models

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestErrNotFoundMatchesThroughWrapping(t *testing.T) {
	wrapped := fmt.Errorf("get person %q: %w", "p-1", ErrNotFound)
	if !errors.Is(wrapped, ErrNotFound) {
		t.Fatal("errors.Is не находит ErrNotFound в обёрнутой ошибке")
	}

	// сопоставление по значению ошибки, а не по тексту
	if errors.Is(errors.New(ErrNotFound.Error()), ErrNotFound) {
		t.Fatal("посторонняя ошибка с тем же текстом не должна совпадать с ErrNotFound")
	}
}

func TestInUseErrorMatchesAsAndNamesReferrers(t *testing.T) {
	err := fmt.Errorf("delete: %w", &InUseError{
		Type: TypePerson, ID: "I-1",
		Referrers: []EntityRef{{Type: TypeRelation, ID: "RL-1"}, {Type: TypeEvent, ID: "E-1"}},
	})

	var inUse *InUseError
	if !errors.As(err, &inUse) {
		t.Fatal("errors.As не находит *InUseError в обёрнутой ошибке")
	}

	for _, want := range []string{`person "I-1"`, "relation RL-1", "event E-1"} {
		if !strings.Contains(inUse.Error(), want) {
			t.Errorf("текст ошибки %q не содержит %q", inUse.Error(), want)
		}
	}

	if MaxReferrers != 20 {
		t.Errorf("MaxReferrers = %d, ожидалось 20", MaxReferrers)
	}
}
