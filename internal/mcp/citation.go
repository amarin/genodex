package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerCitationTools регистрирует тулы для работы с цитатами. source_id —
// обязательная строгая ссылка на источник (просто id). anchor — необязательная
// полиморфная привязка «где именно» (объект, см. anchorObjectProperties) —
// пустой объект или отсутствие аргумента означает «без привязки».
func registerCitationTools(s *server.MCPServer, citations CitationService) {
	tool := mcp.NewTool(
		"citation_list",
		mcp.WithDescription("Список цитат в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, citationListHandler(citations))

	tool = mcp.NewTool(
		"citation_search",
		mcp.WithDescription("Поиск цитат по началу текста; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало текста цитаты")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, citationSearchHandler(citations))

	tool = mcp.NewTool(
		"citation_get",
		mcp.WithDescription("Цитата по id; результат — JSON записи. Неверный формат id, отсутствующая или приватная (без полного доступа) запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например C-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, citationGetHandler(citations))

	tool = mcp.NewTool(
		"citation_create",
		mcp.WithDescription("Создать цитату; id генерируется сервером; результат — JSON созданной записи. Несуществующий source_id или ссылка внутри anchor — ошибка тула"),
		mcp.WithString("source_id", mcp.Required(), mcp.Description("id источника")),
		mcp.WithObject("anchor", mcp.Description("Привязка «где именно» (необязательно)"), mcp.Properties(anchorObjectProperties())),
		mcp.WithString("text", mcp.Description("Текст выписки")),
		mcp.WithString("note", mcp.Description("Заметка")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, citationCreateHandler(citations))

	tool = mcp.NewTool(
		"citation_update",
		mcp.WithDescription("Изменить цитату: полная замена source_id/anchor/text/note/private; результат — JSON обновлённой записи. Несуществующий source_id или ссылка внутри anchor — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("source_id", mcp.Required(), mcp.Description("id источника")),
		mcp.WithObject("anchor", mcp.Description("Привязка «где именно» (необязательно)"), mcp.Properties(anchorObjectProperties())),
		mcp.WithString("text", mcp.Description("Текст выписки")),
		mcp.WithString("note", mcp.Description("Заметка")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, citationUpdateHandler(citations))

	tool = mcp.NewTool(
		"citation_delete",
		mcp.WithDescription("Удалить цитату. Необратимо. Если на неё есть строгие ссылки (SourceLink у любой сущности с доказательствами) — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, citationDeleteHandler(citations))
}

func citationListHandler(citations CitationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := citations.ListCitations(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.CitationsFromModels(list))
	}
}

func citationSearchHandler(citations CitationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := citations.SearchCitations(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.CitationsFromModels(list))
	}
}

func citationGetHandler(citations CitationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		c, err := citations.GetCitation(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.CitationFromModel(c))
	}
}

func citationCreateHandler(citations CitationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		anchor, err := optionalAnchor(req.GetArguments(), "anchor")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		c := models.Citation{
			SourceID: models.ID(req.GetString("source_id", "")),
			Anchor:   anchor,
			Text:     req.GetString("text", ""),
			Note:     req.GetString("note", ""),
			Private:  req.GetBool("private", false),
		}

		created, err := citations.CreateCitation(ctx, c)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.CitationFromModel(created))
	}
}

func citationUpdateHandler(citations CitationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := citations.GetCitation(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		anchor, err := optionalAnchor(req.GetArguments(), "anchor")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.SourceID = models.ID(req.GetString("source_id", ""))
		cur.Anchor = anchor
		cur.Text = req.GetString("text", "")
		cur.Note = req.GetString("note", "")
		cur.Private = req.GetBool("private", false)

		if err := citations.UpdateCitation(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.CitationFromModel(cur))
	}
}

func citationDeleteHandler(citations CitationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := citations.DeleteCitation(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
