package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/amarin/genodex/internal/models"
)

type fakeSettlements struct {
	list []models.AdministrativeDivision
}

func (f fakeSettlements) ListSettlements(context.Context) ([]models.AdministrativeDivision, error) {
	return f.list, nil
}

func TestSettlementListToolContract(t *testing.T) {
	handler := settlementListHandler(fakeSettlements{list: []models.AdministrativeDivision{
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo},
	}})

	res, err := handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	text, ok := mcp.AsTextContent(res.Content[0])
	if !ok {
		t.Fatalf("content[0] = %T, want TextContent", res.Content[0])
	}
	want := `[{"id":"ad-1","name":"Давыдово","type":"selo"}]`
	if text.Text != want {
		t.Fatalf("text = %s, want %s", text.Text, want)
	}
}
