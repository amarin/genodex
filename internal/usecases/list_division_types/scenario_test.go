package list_division_types

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

func TestListDivisionTypes(t *testing.T) {
	got, err := New().ListDivisionTypes(context.Background())
	if err != nil {
		t.Fatalf("ListDivisionTypes: %v", err)
	}

	if len(got) != len(models.AdminDivisionTypes) {
		t.Fatalf("len = %d, ожидалось %d", len(got), len(models.AdminDivisionTypes))
	}

	for i, info := range got {
		if info.Type != models.AdminDivisionTypes[i] {
			t.Errorf("[%d] = %s, ожидался канонический порядок (%s)", i, info.Type, models.AdminDivisionTypes[i])
		}
	}
}
