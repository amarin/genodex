package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerChurchTools регистрирует тулы для работы с церквями. parish —
// одиночная необязательная ссылка (текст или ссылка на приход, объект
// {text, ref?, type?}); ref/type round-trip'ятся как есть (transport.TextRef.
// Model) — клиент, отправляющий обратно ref/type, полученные через
// church_get/list/search, не потеряет ссылку. settlements/notes — списки
// текста (v1, только строки, без ref/type). variants — простые строки.
// ВАЖНО: church_update заменяет переданные списки settlements/notes целиком
// (при отсутствии аргумента в вызове поле сохраняется) — у settlements/notes
// нет ref/type в MCP-контракте вовсе, так что элемент с такой ссылкой
// (заданной иначе, не через MCP) будет потерян при обновлении этого поля
// через MCP, пока не появится picker; parish эту ссылку сохраняет, если её
// передать обратно неизменной.
func registerChurchTools(s *server.MCPServer, churches ChurchService) {
	tool := mcp.NewTool(
		"church_list",
		mcp.WithDescription("Список церквей в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, churchListHandler(churches))

	tool = mcp.NewTool(
		"church_search",
		mcp.WithDescription("Поиск церквей по началу названия; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, churchSearchHandler(churches))

	tool = mcp.NewTool(
		"church_get",
		mcp.WithDescription("Церковь по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например CH-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, churchGetHandler(churches))

	tool = mcp.NewTool(
		"church_create",
		mcp.WithDescription("Создать церковь; id генерируется сервером; результат — JSON созданной записи"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Название")),
		mcp.WithObject("parish", mcp.Description("Приход (текст или ссылка {text, ref?, type?}; ref/type сохраняются, если переданы)"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты (текстом)")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты названия")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
	)
	s.AddTool(tool, churchCreateHandler(churches))

	tool = mcp.NewTool(
		"church_update",
		mcp.WithDescription("Изменить церковь; результат — JSON обновлённой записи. Обновляются переданные поля; при отсутствии аргумента в вызове (кроме обязательного name) соответствующее поле сохраняет текущее значение, явное пустое значение/пустой список — очищает его. parish — {text, ref?, type?}: передайте обратно ref/type, полученные из church_get, чтобы сохранить ссылку; settlements/notes принимают только текст (без ref/type в MCP-контракте) — ссылка на элементе (если задана иначе) будет потеряна при обновлении этого поля через MCP, пока не появится picker (docs/data-model/entity-write.md §5)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Новое название")),
		mcp.WithObject("parish", mcp.Description("Приход (текст или ссылка {text, ref?, type?}; ref/type сохраняются, если переданы)"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты названия")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
	)
	s.AddTool(tool, churchUpdateHandler(churches))

	tool = mcp.NewTool(
		"church_delete",
		mcp.WithDescription("Удалить церковь. Необратимо. Если на неё есть строгие ссылки от других сущностей — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, churchDeleteHandler(churches))
}

func churchListHandler(churches ChurchService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := churches.ListChurches(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.ChurchesFromModels(list))
	}
}

func churchSearchHandler(churches ChurchService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := churches.SearchChurches(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.ChurchesFromModels(list))
	}
}

func churchGetHandler(churches ChurchService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		c, err := churches.GetChurch(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.ChurchFromModel(c))
	}
}

func churchCreateHandler(churches ChurchService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parish, err := optionalTextRef(req.GetArguments(), "parish")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		c := models.Church{
			Name:        req.GetString("name", ""),
			Parish:      parish,
			Settlements: textRefsFromStrings(req.GetStringSlice("settlements", nil)),
			Variants:    req.GetStringSlice("variants", nil),
			Notes:       textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Sources:     sources,
		}

		created, err := churches.CreateChurch(ctx, c)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.ChurchFromModel(created))
	}
}

func churchUpdateHandler(churches ChurchService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := churches.GetChurch(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		cur.Name = req.GetString("name", "")

		if _, ok := args["parish"]; ok {
			parish, err := optionalTextRef(args, "parish")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Parish = parish
		}

		if raw, ok := args["settlements"]; ok && raw != nil {
			cur.Settlements = textRefsFromStrings(req.GetStringSlice("settlements", nil))
		}

		if raw, ok := args["variants"]; ok && raw != nil {
			cur.Variants = req.GetStringSlice("variants", nil)
		}

		if raw, ok := args["notes"]; ok && raw != nil {
			cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		}

		if raw, ok := args["sources"]; ok && raw != nil {
			sources, err := optionalSourceLinks(args, "sources")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Sources = sources
		}

		if err := churches.UpdateChurch(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.ChurchFromModel(cur))
	}
}

func churchDeleteHandler(churches ChurchService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := churches.DeleteChurch(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
