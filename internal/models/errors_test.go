package models

import (
	"errors"
	"fmt"
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
