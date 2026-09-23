package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerGivenNameTools регистрирует тулы для работы со словарными записями
// имён. Запись variants/items/notes — только текстом (v1, как и веб-форма,
// docs/data-model/entity-write.md §4): элемент с уже существующей ссылкой
// (Ref/Type) через эти тулы не создать, только прочитать (list/get/search
// отдают полный TextRef, включая Ref/Type). ВАЖНО: given_name_update заменяет
// списки целиком текстом — существующие ref/type будут потеряны при любом
// обновлении через MCP, пока не появится picker (см. предупреждение в
// описании тула). gender обязателен (models.GivenName.Validate()).
func registerGivenNameTools(s *server.MCPServer, givenNames GivenNameService) {
	tool := mcp.NewTool(
		"given_name_list",
		mcp.WithDescription("Список словарных записей имён в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, givenNameListHandler(givenNames))

	tool = mcp.NewTool(
		"given_name_search",
		mcp.WithDescription("Поиск словарных записей имён по началу канонической формы (включая варианты); результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало канонической формы или варианта")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, givenNameSearchHandler(givenNames))

	tool = mcp.NewTool(
		"given_name_get",
		mcp.WithDescription("Словарная запись имени по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например GN-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, givenNameGetHandler(givenNames))

	tool = mcp.NewTool(
		"given_name_create",
		mcp.WithDescription("Создать словарную запись имени; id генерируется сервером; результат — JSON созданной записи. variants/items/notes — списки текста (без ссылок на другие сущности, v1)"),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Каноническая форма")),
		mcp.WithString("gender", mcp.Required(), mcp.Enum("male", "female", "neutral"),
			mcp.Description("Пол имени — обязателен (male/female/neutral; neutral — Женя, Саша: вывод пола по такому имени запрещён)")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, givenNameCreateHandler(givenNames))

	tool = mcp.NewTool(
		"given_name_update",
		mcp.WithDescription("Изменить словарную запись имени: полная замена canonical/gender/variants/items/notes; результат — JSON обновлённой записи. variants/items/notes передаются целиком как текст — если у элемента раньше была ссылка на другую сущность (ref/type из given_name_get), она будет потеряна: picker для ссылок ещё не реализован ни в вебе, ни в MCP (docs/data-model/entity-write.md §5)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Новая каноническая форма")),
		mcp.WithString("gender", mcp.Required(), mcp.Enum("male", "female", "neutral"),
			mcp.Description("Пол имени — обязателен (male/female/neutral)")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, givenNameUpdateHandler(givenNames))

	tool = mcp.NewTool(
		"given_name_delete",
		mcp.WithDescription("Удалить словарную запись имени. Необратимо. Если на неё есть строгие ссылки от других сущностей — ошибка тула (для GivenName такое сегодня не создаётся, но общий механизм проверки один для всех сущностей)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, givenNameDeleteHandler(givenNames))
}

func givenNameListHandler(givenNames GivenNameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := givenNames.ListGivenNames(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.GivenNamesFromModels(list))
	}
}

func givenNameSearchHandler(givenNames GivenNameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := givenNames.SearchGivenNames(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.GivenNamesFromModels(list))
	}
}

func givenNameGetHandler(givenNames GivenNameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn, err := givenNames.GetGivenName(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.GivenNameFromModel(sn))
	}
}

func givenNameCreateHandler(givenNames GivenNameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn := models.GivenName{
			Canonical: req.GetString("canonical", ""),
			Gender:    models.NameGender(req.GetString("gender", "")),
			Variants:  textRefsFromStrings(req.GetStringSlice("variants", nil)),
			Items:     textRefsFromStrings(req.GetStringSlice("items", nil)),
			Notes:     textRefsFromStrings(req.GetStringSlice("notes", nil)),
		}

		created, err := givenNames.CreateGivenName(ctx, sn)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.GivenNameFromModel(created))
	}
}

func givenNameUpdateHandler(givenNames GivenNameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := givenNames.GetGivenName(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		cur.Canonical = req.GetString("canonical", "")
		cur.Gender = models.NameGender(req.GetString("gender", ""))
		cur.Variants = textRefsFromStrings(req.GetStringSlice("variants", nil))
		cur.Items = textRefsFromStrings(req.GetStringSlice("items", nil))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))

		if err := givenNames.UpdateGivenName(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.GivenNameFromModel(cur))
	}
}

func givenNameDeleteHandler(givenNames GivenNameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := givenNames.DeleteGivenName(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
