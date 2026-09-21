package list_settlements

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type ctxKey struct{}

type fakeRepo struct {
	list   []*models.AdministrativeDivision
	err    error
	gotCtx context.Context
}

func (f *fakeRepo) ListAdministrativeDivisions(ctx context.Context) ([]*models.AdministrativeDivision, error) {
	f.gotCtx = ctx

	return f.list, f.err
}

func TestScenarioListSettlementsFiltersDivisions(t *testing.T) {
	sc := New(&fakeRepo{
		list: []*models.AdministrativeDivision{
			{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo},
			{ID: "ad-2", Name: "Никифоровская", Type: models.AdminDivisionVolost},
			{ID: "ad-3", Name: "Никифорово", Type: models.AdminDivisionDerevnya},
		},
	})

	got, err := sc.ListSettlements(context.Background())
	if err != nil {
		t.Fatalf("ListSettlements: %v", err)
	}
	if len(got) != 2 || got[0].ID != "ad-1" || got[1].ID != "ad-3" {
		t.Fatalf("got %+v, want ad-1 и ad-3 (волость отфильтрована)", got)
	}
}

func TestScenarioPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")
	sc := New(&fakeRepo{err: wantErr})

	if _, err := sc.ListSettlements(context.Background()); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}
