package mcp

import (
	"github.com/mark3labs/mcp-go/server"
)

// Deps — сервисы, отдаваемые в MCP-тулы. Явный реестр вместо растущего
// списка позиционных параметров (docs/data-model/entity-write.md §3) — новая
// сущность добавляется полем структуры, не меняя сигнатуру NewServer.
type Deps struct {
	Divisions    DivisionService
	Surnames     SurnameService
	Patronymics  PatronymicService
	Estates      EstateService
	Titles       TitleService
	GivenNames   GivenNameService
	Repositories RepositoryService
	Churches     ChurchService
	Parishes     ParishService
	Archives     ArchiveService
	Notes        NoteService
	Attachments  AttachmentService
}

// NewServer создаёт MCP-сервер и регистрирует доступные тулы.
func NewServer(deps Deps) *server.MCPServer {
	s := server.NewMCPServer(
		"genodex",
		"0.1.0",
	)

	registerDivisionTools(s, deps.Divisions)

	if deps.Surnames != nil {
		registerSurnameTools(s, deps.Surnames)
	}

	if deps.Patronymics != nil {
		registerPatronymicTools(s, deps.Patronymics)
	}

	if deps.Estates != nil {
		registerEstateTools(s, deps.Estates)
	}

	if deps.Titles != nil {
		registerTitleTools(s, deps.Titles)
	}

	if deps.GivenNames != nil {
		registerGivenNameTools(s, deps.GivenNames)
	}

	if deps.Repositories != nil {
		registerRepositoryTools(s, deps.Repositories)
	}

	if deps.Churches != nil {
		registerChurchTools(s, deps.Churches)
	}

	if deps.Parishes != nil {
		registerParishTools(s, deps.Parishes)
	}

	if deps.Archives != nil {
		registerArchiveTools(s, deps.Archives)
	}

	if deps.Notes != nil {
		registerNoteTools(s, deps.Notes)
	}

	if deps.Attachments != nil {
		registerAttachmentTools(s, deps.Attachments)
	}

	return s
}
