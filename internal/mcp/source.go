package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerSourceTools регистрирует тулы для работы с источниками
// доказательств. date — структурированная дата (объект, см.
// factDateObjectProperties), по образцу Parish.Since/Until. repository_id —
// просто id (не объект TextRef): пустая строка — без хранилища; сценарий
// проверяет существование при непустом значении.
func registerSourceTools(s *server.MCPServer, sources SourceService) {
	tool := mcp.NewTool(
		"source_list",
		mcp.WithDescription("Список источников в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, sourceListHandler(sources))

	tool = mcp.NewTool(
		"source_search",
		mcp.WithDescription("Поиск источников по началу названия или автора; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия или автора")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, sourceSearchHandler(sources))

	tool = mcp.NewTool(
		"source_get",
		mcp.WithDescription("Источник по id; результат — JSON записи. Неверный формат id, отсутствующая или приватная (без полного доступа) запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например S-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, sourceGetHandler(sources))

	tool = mcp.NewTool(
		"source_create",
		mcp.WithDescription("Создать источник; id генерируется сервером; результат — JSON созданной записи. Несуществующий repository_id — ошибка тула"),
		mcp.WithString("kind", mcp.Required(), mcp.Enum("archival-scan", "transcription", "document", "audio", "photo", "memory", "external"), mcp.Description("Вид источника")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Название")),
		mcp.WithString("author", mcp.Description("Автор")),
		mcp.WithObject("date", mcp.Description("Дата источника"), mcp.Properties(factDateObjectProperties())),
		mcp.WithString("reliability", mcp.Required(), mcp.Enum("primary", "contemporary", "memory", "indirect", "unknown"), mcp.Description("Общая достоверность")),
		mcp.WithString("repository_id", mcp.Description("id хранилища (необязательно; пусто — без хранилища)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, sourceCreateHandler(sources))

	tool = mcp.NewTool(
		"source_update",
		mcp.WithDescription("Изменить источник: kind/title/reliability — обязательные поля, заменяются всегда; author/date/repository_id/notes/private — при отсутствии аргумента в вызове сохраняют текущее значение, явное пустое значение/пустой список — очищает его; результат — JSON обновлённой записи. Несуществующий repository_id — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("kind", mcp.Required(), mcp.Enum("archival-scan", "transcription", "document", "audio", "photo", "memory", "external"), mcp.Description("Вид источника")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Название")),
		mcp.WithString("author", mcp.Description("Автор")),
		mcp.WithObject("date", mcp.Description("Дата источника"), mcp.Properties(factDateObjectProperties())),
		mcp.WithString("reliability", mcp.Required(), mcp.Enum("primary", "contemporary", "memory", "indirect", "unknown"), mcp.Description("Общая достоверность")),
		mcp.WithString("repository_id", mcp.Description("id хранилища (необязательно)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, sourceUpdateHandler(sources))

	tool = mcp.NewTool(
		"source_delete",
		mcp.WithDescription("Удалить источник. Необратимо. Если на него есть строгие ссылки от других сущностей (например, Citation) — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, sourceDeleteHandler(sources))
}

func sourceListHandler(sources SourceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := sources.ListSources(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.SourcesFromModels(list))
	}
}

func sourceSearchHandler(sources SourceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := sources.SearchSources(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.SourcesFromModels(list))
	}
}

func sourceGetHandler(sources SourceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		src, err := sources.GetSource(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.SourceFromModel(src))
	}
}

func sourceCreateHandler(sources SourceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		date, err := optionalFactDate(req.GetArguments(), "date")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		src := models.Source{
			Kind:         models.SourceKind(req.GetString("kind", "")),
			Title:        req.GetString("title", ""),
			Author:       req.GetString("author", ""),
			Date:         date,
			Reliability:  models.Reliability(req.GetString("reliability", "")),
			RepositoryID: models.ID(req.GetString("repository_id", "")),
			Notes:        textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Private:      req.GetBool("private", false),
		}

		created, err := sources.CreateSource(ctx, src)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.SourceFromModel(created))
	}
}

func sourceUpdateHandler(sources SourceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := sources.GetSource(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		cur.Kind = models.SourceKind(req.GetString("kind", ""))
		cur.Title = req.GetString("title", "")
		cur.Reliability = models.Reliability(req.GetString("reliability", ""))

		if raw, ok := args["author"]; ok && raw != nil {
			cur.Author = req.GetString("author", "")
		}

		if raw, ok := args["date"]; ok && raw != nil {
			date, err := optionalFactDate(args, "date")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Date = date
		}

		if raw, ok := args["repository_id"]; ok && raw != nil {
			cur.RepositoryID = models.ID(req.GetString("repository_id", ""))
		}

		if raw, ok := args["notes"]; ok && raw != nil {
			cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		}

		if raw, ok := args["private"]; ok && raw != nil {
			cur.Private = req.GetBool("private", false)
		}

		if err := sources.UpdateSource(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.SourceFromModel(cur))
	}
}

func sourceDeleteHandler(sources SourceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := sources.DeleteSource(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
