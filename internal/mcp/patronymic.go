package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerPatronymicTools регистрирует тулы для работы со словарными записями
// отчеств. Запись variants/items/notes — только текстом (v1, как и веб-форма,
// docs/data-model/entity-write.md §4): элемент с уже существующей ссылкой
// (Ref/Type) через эти тулы не создать, только прочитать (list/get/search
// отдают полный TextRef, включая Ref/Type). ВАЖНО: patronymic_update заменяет
// списки целиком текстом — существующие ref/type будут потеряны при любом
// обновлении через MCP, пока не появится picker (см. предупреждение в
// описании тула).
func registerPatronymicTools(s *server.MCPServer, patronymics PatronymicService) {
	tool := mcp.NewTool(
		"patronymic_list",
		mcp.WithDescription("Список словарных записей отчеств в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, patronymicListHandler(patronymics))

	tool = mcp.NewTool(
		"patronymic_search",
		mcp.WithDescription("Поиск словарных записей отчеств по началу канонической формы (включая варианты); результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало канонической формы или варианта")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, patronymicSearchHandler(patronymics))

	tool = mcp.NewTool(
		"patronymic_get",
		mcp.WithDescription("Словарная запись отчества по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например PN-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, patronymicGetHandler(patronymics))

	tool = mcp.NewTool(
		"patronymic_create",
		mcp.WithDescription("Создать словарную запись отчества; id генерируется сервером; результат — JSON созданной записи. variants/items/notes — списки текста (без ссылок на другие сущности, v1)"),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Каноническая форма")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, patronymicCreateHandler(patronymics))

	tool = mcp.NewTool(
		"patronymic_update",
		mcp.WithDescription("Изменить словарную запись отчества: canonical — обязательное поле, заменяется всегда; variants/items/notes — при отсутствии аргумента в вызове сохраняют текущее значение, явный пустой список — очищает его; результат — JSON обновлённой записи. variants/items/notes передаются целиком как текст — если у элемента раньше была ссылка на другую сущность (ref/type из patronymic_get), она будет потеряна: picker для ссылок ещё не реализован ни в вебе, ни в MCP (docs/data-model/entity-write.md §5)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Новая каноническая форма")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, patronymicUpdateHandler(patronymics))

	tool = mcp.NewTool(
		"patronymic_delete",
		mcp.WithDescription("Удалить словарную запись отчества. Необратимо. Если на неё есть строгие ссылки от других сущностей — ошибка тула (для Patronymic такое сегодня не создаётся, но общий механизм проверки один для всех сущностей)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, patronymicDeleteHandler(patronymics))
}

func patronymicListHandler(patronymics PatronymicService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := patronymics.ListPatronymics(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.PatronymicsFromModels(list))
	}
}

func patronymicSearchHandler(patronymics PatronymicService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := patronymics.SearchPatronymics(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.PatronymicsFromModels(list))
	}
}

func patronymicGetHandler(patronymics PatronymicService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn, err := patronymics.GetPatronymic(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.PatronymicFromModel(sn))
	}
}

func patronymicCreateHandler(patronymics PatronymicService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn := models.Patronymic{
			Canonical: req.GetString("canonical", ""),
			Variants:  textRefsFromStrings(req.GetStringSlice("variants", nil)),
			Items:     textRefsFromStrings(req.GetStringSlice("items", nil)),
			Notes:     textRefsFromStrings(req.GetStringSlice("notes", nil)),
		}

		created, err := patronymics.CreatePatronymic(ctx, sn)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.PatronymicFromModel(created))
	}
}

func patronymicUpdateHandler(patronymics PatronymicService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := patronymics.GetPatronymic(ctx, id)
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

		if err := patronymics.UpdatePatronymic(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.PatronymicFromModel(cur))
	}
}

func patronymicDeleteHandler(patronymics PatronymicService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := patronymics.DeletePatronymic(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
