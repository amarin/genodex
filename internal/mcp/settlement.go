package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/store"
)

// registerSettlementTools регистрирует тулы для работы с населёнными пунктами.
func registerSettlementTools(s *server.MCPServer, st *store.Store) {
	tool := mcp.NewTool(
		"settlement_list",
		mcp.WithDescription("Получить список всех населённых пунктов"),
	)

	s.AddTool(tool, settlementListHandler(st))
}

// settlementListHandler возвращает обработчик тула settlement_list.
//
// Пример обработчика: получает аргументы из запроса, обращается к
// хранилищу и возвращает результат. Ошибки тула сообщаются внутри
// CallToolResult через NewToolResultError.
func settlementListHandler(st *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		settlements := st.ListSettlements()

		data, err := json.Marshal(settlements)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сериализовать список: %v", err)), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}
