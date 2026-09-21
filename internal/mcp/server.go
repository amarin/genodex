package mcp

import (
	"github.com/mark3labs/mcp-go/server"
)

// NewServer создаёт MCP-сервер и регистрирует доступные тулы.
func NewServer(divisions DivisionService) *server.MCPServer {
	s := server.NewMCPServer(
		"genodex",
		"0.1.0",
	)

	registerDivisionTools(s, divisions)

	return s
}
