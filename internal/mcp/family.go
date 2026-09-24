package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerFamilyTools регистрирует тулы для работы с родами/линиями.
// members/notes — только текстом (v1, docs/data-model/entity-write.md §4).
// ВАЖНО: family_update заменяет списки целиком текстом — существующие
// ref/type будут потеряны при любом обновлении через MCP, пока не появится
// picker.
func registerFamilyTools(s *server.MCPServer, families FamilyService) {
	tool := mcp.NewTool(
		"family_list",
		mcp.WithDescription("Список родов/линий в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, familyListHandler(families))

	tool = mcp.NewTool(
		"family_search",
		mcp.WithDescription("Поиск родов по началу названия; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, familySearchHandler(families))

	tool = mcp.NewTool(
		"family_get",
		mcp.WithDescription("Род по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула; то же для рода, ссылающегося на приватную цитату среди источников (sources[i].citation_id)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например F-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, familyGetHandler(families))

	tool = mcp.NewTool(
		"family_create",
		mcp.WithDescription("Создать род; id генерируется сервером; результат — JSON созданной записи"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Название")),
		mcp.WithArray("members", mcp.WithStringItems(), mcp.Description("Члены рода (текстом; мягкая ссылка на персону, без проверки существования)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, familyCreateHandler(families))

	tool = mcp.NewTool(
		"family_update",
		mcp.WithDescription("Изменить род: обновляются переданные поля; при отсутствии аргумента в вызове соответствующее поле (members/notes/sources/private) сохраняет текущее значение, явное пустое значение/пустой список — очищает его; результат — JSON обновлённой записи"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Новое название")),
		mcp.WithArray("members", mcp.WithStringItems(), mcp.Description("Члены рода")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, familyUpdateHandler(families))

	tool = mcp.NewTool(
		"family_delete",
		mcp.WithDescription("Удалить род. Необратимо. Если на него есть строгие ссылки — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, familyDeleteHandler(families))
}

func familyListHandler(families FamilyService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := families.ListFamilies(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.FamiliesFromModels(list))
	}
}

func familySearchHandler(families FamilyService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := families.SearchFamilies(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.FamiliesFromModels(list))
	}
}

func familyGetHandler(families FamilyService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		f, err := families.GetFamily(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.FamilyFromModel(f))
	}
}

func familyCreateHandler(families FamilyService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		f := models.Family{
			Name:    req.GetString("name", ""),
			Members: textRefsFromStrings(req.GetStringSlice("members", nil)),
			Notes:   textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Sources: sources,
			Private: req.GetBool("private", false),
		}

		created, err := families.CreateFamily(ctx, f)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.FamilyFromModel(created))
	}
}

func familyUpdateHandler(families FamilyService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := families.GetFamily(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		cur.Name = req.GetString("name", "")

		if raw, ok := args["members"]; ok && raw != nil {
			cur.Members = textRefsFromStrings(req.GetStringSlice("members", nil))
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

		if err := families.UpdateFamily(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.FamilyFromModel(cur))
	}
}

func familyDeleteHandler(families FamilyService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := families.DeleteFamily(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
