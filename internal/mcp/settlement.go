package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/transport"
)

// registerSettlementTools регистрирует тулы для работы с населёнными пунктами.
func registerSettlementTools(s *server.MCPServer, settlements SettlementService) {
	tool := mcp.NewTool(
		"settlement_list",
		mcp.WithDescription("Получить список всех населённых пунктов"),
	)

	s.AddTool(tool, settlementListHandler(settlements))
}

// settlementListHandler возвращает обработчик тула settlement_list.
//
// Пример обработчика: получает аргументы из запроса, обращается к
// сценарию и возвращает результат. Ошибки тула сообщаются внутри
// CallToolResult через NewToolResultError.
func settlementListHandler(settlements SettlementService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		list, err := settlements.ListSettlements(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		data, err := json.Marshal(transport.SettlementsFromModels(list))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сериализовать список: %v", err)), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}
