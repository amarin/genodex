package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerRepositoryTools регистрирует тулы для работы с хранилищами-
// контейнерами источников. urls/notes — только текстом (v1,
// docs/data-model/entity-write.md §4). ВАЖНО: repository_update заменяет
// списки целиком текстом — существующие ref/type будут потеряны при любом
// обновлении через MCP, пока не появится picker.
func registerRepositoryTools(s *server.MCPServer, repositories RepositoryService) {
	tool := mcp.NewTool(
		"repository_list",
		mcp.WithDescription("Список хранилищ-контейнеров источников в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, repositoryListHandler(repositories))

	tool = mcp.NewTool(
		"repository_search",
		mcp.WithDescription("Поиск хранилищ по началу названия; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, repositorySearchHandler(repositories))

	tool = mcp.NewTool(
		"repository_get",
		mcp.WithDescription("Хранилище по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например R-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, repositoryGetHandler(repositories))

	tool = mcp.NewTool(
		"repository_create",
		mcp.WithDescription("Создать хранилище; id генерируется сервером; результат — JSON созданной записи"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Название")),
		mcp.WithString("type", mcp.Required(), mcp.Description("Тип хранилища: открытый список, формат [a-z][a-z0-9_-]* (archive/library/museum/private/other — типовые значения, допустимы и другие)")),
		mcp.WithString("address", mcp.Description("Адрес")),
		mcp.WithArray("urls", mcp.WithStringItems(), mcp.Description("Ссылки (URL/DOI и т.п., текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, repositoryCreateHandler(repositories))

	tool = mcp.NewTool(
		"repository_update",
		mcp.WithDescription("Изменить хранилище: name/type — обязательные поля, заменяются всегда; address/urls/notes/sources/private — при отсутствии аргумента в вызове сохраняют текущее значение, явное пустое значение/пустой список — очищает его; результат — JSON обновлённой записи"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Новое название")),
		mcp.WithString("type", mcp.Required(), mcp.Description("Новый тип")),
		mcp.WithString("address", mcp.Description("Адрес")),
		mcp.WithArray("urls", mcp.WithStringItems(), mcp.Description("Ссылки")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, repositoryUpdateHandler(repositories))

	tool = mcp.NewTool(
		"repository_delete",
		mcp.WithDescription("Удалить хранилище. Необратимо. Если на него есть строгие ссылки (Archive.repository_id) — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, repositoryDeleteHandler(repositories))
}

func repositoryListHandler(repositories RepositoryService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := repositories.ListRepositories(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.RepositoriesFromModels(list))
	}
}

func repositorySearchHandler(repositories RepositoryService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := repositories.SearchRepositories(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.RepositoriesFromModels(list))
	}
}

func repositoryGetHandler(repositories RepositoryService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		r, err := repositories.GetRepository(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.RepositoryFromModel(r))
	}
}

func repositoryCreateHandler(repositories RepositoryService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		r := models.Repository{
			Name:    req.GetString("name", ""),
			Type:    models.RepositoryType(req.GetString("type", "")),
			Address: req.GetString("address", ""),
			URLs:    textRefsFromStrings(req.GetStringSlice("urls", nil)),
			Notes:   textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Sources: sources,
			Private: req.GetBool("private", false),
		}

		created, err := repositories.CreateRepository(ctx, r)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.RepositoryFromModel(created))
	}
}

func repositoryUpdateHandler(repositories RepositoryService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := repositories.GetRepository(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		cur.Name = req.GetString("name", "")
		cur.Type = models.RepositoryType(req.GetString("type", ""))

		if raw, ok := args["address"]; ok && raw != nil {
			cur.Address = req.GetString("address", "")
		}

		if raw, ok := args["urls"]; ok && raw != nil {
			cur.URLs = textRefsFromStrings(req.GetStringSlice("urls", nil))
		}

		if raw, ok := args["notes"]; ok && raw != nil {
			cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		}

		if raw, ok := args["private"]; ok && raw != nil {
			cur.Private = req.GetBool("private", false)
		}

		if raw, ok := args["sources"]; ok && raw != nil {
			sources, err := optionalSourceLinks(args, "sources")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Sources = sources
		}

		if err := repositories.UpdateRepository(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.RepositoryFromModel(cur))
	}
}

func repositoryDeleteHandler(repositories RepositoryService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := repositories.DeleteRepository(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
