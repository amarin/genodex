package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerParishTools регистрирует тулы для работы с приходами. church —
// одиночная необязательная ссылка (текст или ссылка на церковь, объект
// {text, ref?, type?}); ref/type round-trip'ятся как есть (transport.TextRef.
// Model) — клиент, отправляющий обратно ref/type, полученные через
// parish_get/list/search, не потеряет ссылку. since/until — структурированная
// дата (объект, см. factDateObjectProperties). ВАЖНО: parish_update заменяет
// church/settlements/notes целиком — у settlements/notes нет ref/type в
// MCP-контракте вовсе (только текст), так что такая ссылка (заданная иначе)
// будет потеряна при любом обновлении через MCP, пока не появится picker;
// church эту ссылку сохраняет, если её передать обратно неизменной.
func registerParishTools(s *server.MCPServer, parishes ParishService) {
	tool := mcp.NewTool(
		"parish_list",
		mcp.WithDescription("Список приходов в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, parishListHandler(parishes))

	tool = mcp.NewTool(
		"parish_search",
		mcp.WithDescription("Поиск приходов по началу названия; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, parishSearchHandler(parishes))

	tool = mcp.NewTool(
		"parish_get",
		mcp.WithDescription("Приход по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например PR-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, parishGetHandler(parishes))

	tool = mcp.NewTool(
		"parish_create",
		mcp.WithDescription("Создать приход; id генерируется сервером; результат — JSON созданной записи"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Название")),
		mcp.WithObject("church", mcp.Description("Церковь (текст или ссылка {text, ref?, type?}; ref/type сохраняются, если переданы)"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты (текстом)")),
		mcp.WithObject("since", mcp.Description("Начало периода действия прихода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец периода действия прихода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
	)
	s.AddTool(tool, parishCreateHandler(parishes))

	tool = mcp.NewTool(
		"parish_update",
		mcp.WithDescription("Изменить приход: полная замена name/church/settlements/since/until/notes; результат — JSON обновлённой записи. church — {text, ref?, type?}: передайте обратно ref/type, полученные из parish_get, чтобы сохранить ссылку; settlements/notes принимают только текст (без ref/type в MCP-контракте) — ссылка на элементе (если задана иначе) будет потеряна при любом обновлении через MCP, пока не появится picker (docs/data-model/entity-write.md §5)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Новое название")),
		mcp.WithObject("church", mcp.Description("Церковь (текст или ссылка {text, ref?, type?}; ref/type сохраняются, если переданы)"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты")),
		mcp.WithObject("since", mcp.Description("Начало периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
	)
	s.AddTool(tool, parishUpdateHandler(parishes))

	tool = mcp.NewTool(
		"parish_delete",
		mcp.WithDescription("Удалить приход. Необратимо. Если на него есть строгие ссылки от других сущностей — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, parishDeleteHandler(parishes))
}

func parishListHandler(parishes ParishService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := parishes.ListParishes(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.ParishesFromModels(list))
	}
}

func parishSearchHandler(parishes ParishService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := parishes.SearchParishes(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.ParishesFromModels(list))
	}
}

func parishGetHandler(parishes ParishService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		p, err := parishes.GetParish(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.ParishFromModel(p))
	}
}

func parishCreateHandler(parishes ParishService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		church, err := optionalTextRef(args, "church")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		since, err := optionalFactDate(args, "since")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		until, err := optionalFactDate(args, "until")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sources, err := optionalSourceLinks(args, "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		p := models.Parish{
			Name:        req.GetString("name", ""),
			Church:      church,
			Settlements: textRefsFromStrings(req.GetStringSlice("settlements", nil)),
			Since:       since,
			Until:       until,
			Notes:       textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Sources:     sources,
		}

		created, err := parishes.CreateParish(ctx, p)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.ParishFromModel(created))
	}
}

func parishUpdateHandler(parishes ParishService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := parishes.GetParish(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		church, err := optionalTextRef(args, "church")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		since, err := optionalFactDate(args, "since")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		until, err := optionalFactDate(args, "until")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sources, err := optionalSourceLinks(args, "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.Name = req.GetString("name", "")
		cur.Church = church
		cur.Settlements = textRefsFromStrings(req.GetStringSlice("settlements", nil))
		cur.Since = since
		cur.Until = until
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		cur.Sources = sources

		if err := parishes.UpdateParish(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.ParishFromModel(cur))
	}
}

func parishDeleteHandler(parishes ParishService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := parishes.DeleteParish(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
