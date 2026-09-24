package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerTitleTools регистрирует тулы для работы со словарными записями
// титулов. Запись variants/items/notes — только текстом (v1, как и веб-форма,
// docs/data-model/entity-write.md §4): элемент с уже существующей ссылкой
// (Ref/Type) через эти тулы не создать, только прочитать (list/get/search
// отдают полный TextRef, включая Ref/Type). ВАЖНО: title_update заменяет
// списки целиком текстом — существующие ref/type будут потеряны при любом
// обновлении через MCP, пока не появится picker (см. предупреждение в
// описании тула).
func registerTitleTools(s *server.MCPServer, titles TitleService) {
	tool := mcp.NewTool(
		"title_list",
		mcp.WithDescription("Список словарных записей титулов в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, titleListHandler(titles))

	tool = mcp.NewTool(
		"title_search",
		mcp.WithDescription("Поиск словарных записей титулов по началу канонической формы (включая варианты); результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало канонической формы или варианта")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, titleSearchHandler(titles))

	tool = mcp.NewTool(
		"title_get",
		mcp.WithDescription("Словарная запись титула по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например TT-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, titleGetHandler(titles))

	tool = mcp.NewTool(
		"title_create",
		mcp.WithDescription("Создать словарную запись титула; id генерируется сервером; результат — JSON созданной записи. variants/items/notes — списки текста (без ссылок на другие сущности, v1)"),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Каноническая форма")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, titleCreateHandler(titles))

	tool = mcp.NewTool(
		"title_update",
		mcp.WithDescription("Изменить словарную запись титула: canonical — обязательное поле, заменяется всегда; variants/items/notes — при отсутствии аргумента в вызове сохраняют текущее значение, явный пустой список — очищает его; результат — JSON обновлённой записи. variants/items/notes передаются целиком как текст — если у элемента раньше была ссылка на другую сущность (ref/type из title_get), она будет потеряна: picker для ссылок ещё не реализован ни в вебе, ни в MCP (docs/data-model/entity-write.md §5)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Новая каноническая форма")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, titleUpdateHandler(titles))

	tool = mcp.NewTool(
		"title_delete",
		mcp.WithDescription("Удалить словарную запись титула. Необратимо. Если на неё есть строгие ссылки от других сущностей — ошибка тула (для Title такое сегодня не создаётся, но общий механизм проверки один для всех сущностей)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, titleDeleteHandler(titles))
}

func titleListHandler(titles TitleService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := titles.ListTitles(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.TitlesFromModels(list))
	}
}

func titleSearchHandler(titles TitleService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := titles.SearchTitles(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.TitlesFromModels(list))
	}
}

func titleGetHandler(titles TitleService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn, err := titles.GetTitle(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.TitleFromModel(sn))
	}
}

func titleCreateHandler(titles TitleService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn := models.Title{
			Canonical: req.GetString("canonical", ""),
			Variants:  textRefsFromStrings(req.GetStringSlice("variants", nil)),
			Items:     textRefsFromStrings(req.GetStringSlice("items", nil)),
			Notes:     textRefsFromStrings(req.GetStringSlice("notes", nil)),
		}

		created, err := titles.CreateTitle(ctx, sn)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.TitleFromModel(created))
	}
}

func titleUpdateHandler(titles TitleService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := titles.GetTitle(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		cur.Canonical = req.GetString("canonical", "")

		if raw, ok := args["variants"]; ok && raw != nil {
			cur.Variants = textRefsFromStrings(req.GetStringSlice("variants", nil))
		}

		if raw, ok := args["items"]; ok && raw != nil {
			cur.Items = textRefsFromStrings(req.GetStringSlice("items", nil))
		}

		if raw, ok := args["notes"]; ok && raw != nil {
			cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		}

		if err := titles.UpdateTitle(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.TitleFromModel(cur))
	}
}

func titleDeleteHandler(titles TitleService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := titles.DeleteTitle(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
