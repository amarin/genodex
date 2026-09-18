package mcp

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/store"
)

// NewServer создаёт MCP-сервер и регистрирует доступные тулы.
func NewServer(st *store.Store) *server.MCPServer {
	s := server.NewMCPServer(
		"genodex",
		"0.1.0",
	)

	registerSettlementTools(s, st)

	return s
}
