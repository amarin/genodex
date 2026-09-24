package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/amarin/genodex/internal/transport"
)

func TestDivisionTypeListTool(t *testing.T) {
	res, err := divisionTypeListHandler(&fakeDivisions{})(context.Background(), mcp.CallToolRequest{})
	if err != nil || res.IsError {
		t.Fatalf("err=%v isError=%v", err, res != nil && res.IsError)
	}

	var got []transport.AdminDivisionTypeInfo
	if err := json.Unmarshal([]byte(resultText(t, res)), &got); err != nil {
		t.Fatalf("результат не JSON-массив: %v", err)
	}

	for _, info := range got {
		if info.Type == "uezd" {
			if info.Label != "Уезд" || info.Rank == nil || *info.Rank != 70 || len(info.Children) == 0 {
				t.Errorf("uezd = %+v", info)
			}

			return
		}
	}

	t.Fatal("в справочнике нет uezd")
}
