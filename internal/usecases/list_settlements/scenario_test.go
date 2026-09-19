package list_settlements

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/entity"
)

type fakeRepo struct {
	list []*entity.Settlement
	err  error
}

func (f *fakeRepo) ListSettlements() ([]*entity.Settlement, error) {
	return f.list, f.err
}

func TestScenarioListSettlements(t *testing.T) {
	sc := New(&fakeRepo{
		list: []*entity.Settlement{
			{ID: "sett-1", Name: "Давыдово"},
			{ID: "sett-2", Name: "Никифорово", Metadata: map[string]string{"volost_id": "v-1"}},
		},
	})

	got, err := sc.ListSettlements(context.Background())
	if err != nil {
		t.Fatalf("ListSettlements: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2", len(got))
	}
	if got[0].ID != "sett-1" || got[0].Name != "Давыдово" {
		t.Errorf("got[0]=%+v", got[0])
	}
	if got[1].Metadata["volost_id"] != "v-1" {
		t.Errorf("got[1].Metadata=%v", got[1].Metadata)
	}
}

func TestScenarioPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")
	sc := New(&fakeRepo{err: wantErr})

	if _, err := sc.ListSettlements(context.Background()); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}
