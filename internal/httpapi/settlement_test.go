package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeSettlements struct {
	list []models.AdministrativeDivision
	err  error
}

func (f fakeSettlements) ListSettlements(context.Context) ([]models.AdministrativeDivision, error) {
	return f.list, f.err
}

func TestSettlementListContract(t *testing.T) {
	h := NewHandler(fakeSettlements{list: []models.AdministrativeDivision{
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo},
	}}, fstest.MapFS{})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/settlements", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	want := `[{"id":"ad-1","name":"Давыдово","type":"selo"}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}
