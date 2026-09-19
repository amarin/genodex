package mcp

import (
	"github.com/mark3labs/mcp-go/server"
)

// NewServer создаёт MCP-сервер и регистрирует доступные тулы.
func NewServer(settlements SettlementService) *server.MCPServer {
	s := server.NewMCPServer(
		"genodex",
		"0.1.0",
	)

	registerSettlementTools(s, settlements)

	return s
}
