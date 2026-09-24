package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerSurnameTools регистрирует тулы для работы со словарными записями
// фамилий. Запись variants/items/notes — только текстом (v1, как и веб-форма,
// docs/data-model/entity-write.md §4): элемент с уже существующей ссылкой
// (Ref/Type) через эти тулы не создать, только прочитать (list/get/search
// отдают полный TextRef, включая Ref/Type). ВАЖНО: surname_update заменяет
// списки целиком текстом — существующие ref/type будут потеряны при любом
// обновлении через MCP, пока не появится picker (см. предупреждение в
// описании тула).
func registerSurnameTools(s *server.MCPServer, surnames SurnameService) {
	tool := mcp.NewTool(
		"surname_list",
		mcp.WithDescription("Список словарных записей фамилий в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, surnameListHandler(surnames))

	tool = mcp.NewTool(
		"surname_search",
		mcp.WithDescription("Поиск словарных записей фамилий по началу канонической формы (включая варианты); результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало канонической формы или варианта")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, surnameSearchHandler(surnames))

	tool = mcp.NewTool(
		"surname_get",
		mcp.WithDescription("Словарная запись фамилии по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например SN-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, surnameGetHandler(surnames))

	tool = mcp.NewTool(
		"surname_create",
		mcp.WithDescription("Создать словарную запись фамилии; id генерируется сервером; результат — JSON созданной записи. variants/items/notes — списки текста (без ссылок на другие сущности, v1)"),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Каноническая форма")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, surnameCreateHandler(surnames))

	tool = mcp.NewTool(
		"surname_update",
		mcp.WithDescription("Изменить словарную запись фамилии: canonical — обязательное поле, заменяется всегда; variants/items/notes — при отсутствии аргумента в вызове сохраняют текущее значение, явный пустой список — очищает его; результат — JSON обновлённой записи. variants/items/notes передаются целиком как текст — если у элемента раньше была ссылка на другую сущность (ref/type из surname_get), она будет потеряна: picker для ссылок ещё не реализован ни в вебе, ни в MCP (docs/data-model/entity-write.md §5)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Новая каноническая форма")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, surnameUpdateHandler(surnames))

	tool = mcp.NewTool(
		"surname_delete",
		mcp.WithDescription("Удалить словарную запись фамилии. Необратимо. Если на неё есть строгие ссылки от других сущностей — ошибка тула (для Surname такое сегодня не создаётся, но общий механизм проверки один для всех сущностей)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, surnameDeleteHandler(surnames))
}

func surnameListHandler(surnames SurnameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := surnames.ListSurnames(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.SurnamesFromModels(list))
	}
}

func surnameSearchHandler(surnames SurnameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := surnames.SearchSurnames(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.SurnamesFromModels(list))
	}
}

func surnameGetHandler(surnames SurnameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn, err := surnames.GetSurname(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.SurnameFromModel(sn))
	}
}

// textRefsFromStrings — v1 текстовые списки MCP-тулов в []models.TextRef без Ref/Type.
func textRefsFromStrings(ss []string) []models.TextRef {
	out := make([]models.TextRef, 0, len(ss))
	for _, s := range ss {
		out = append(out, models.TextRef{Text: s})
	}

	return out
}

func surnameCreateHandler(surnames SurnameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn := models.Surname{
			Canonical: req.GetString("canonical", ""),
			Variants:  textRefsFromStrings(req.GetStringSlice("variants", nil)),
			Items:     textRefsFromStrings(req.GetStringSlice("items", nil)),
			Notes:     textRefsFromStrings(req.GetStringSlice("notes", nil)),
		}

		created, err := surnames.CreateSurname(ctx, sn)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.SurnameFromModel(created))
	}
}

func surnameUpdateHandler(surnames SurnameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := surnames.GetSurname(ctx, id)
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

		if err := surnames.UpdateSurname(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.SurnameFromModel(cur))
	}
}

func surnameDeleteHandler(surnames SurnameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := surnames.DeleteSurname(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
