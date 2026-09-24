package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerEstateTools регистрирует тулы для работы со словарными записями
// сословий. Запись variants/items/notes — только текстом (v1, как и веб-форма,
// docs/data-model/entity-write.md §4): элемент с уже существующей ссылкой
// (Ref/Type) через эти тулы не создать, только прочитать (list/get/search
// отдают полный TextRef, включая Ref/Type). ВАЖНО: estate_update заменяет
// списки целиком текстом — существующие ref/type будут потеряны при любом
// обновлении через MCP, пока не появится picker (см. предупреждение в
// описании тула).
func registerEstateTools(s *server.MCPServer, estates EstateService) {
	tool := mcp.NewTool(
		"estate_list",
		mcp.WithDescription("Список словарных записей сословий в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, estateListHandler(estates))

	tool = mcp.NewTool(
		"estate_search",
		mcp.WithDescription("Поиск словарных записей сословий по началу канонической формы (включая варианты); результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало канонической формы или варианта")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, estateSearchHandler(estates))

	tool = mcp.NewTool(
		"estate_get",
		mcp.WithDescription("Словарная запись сословия по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например ES-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, estateGetHandler(estates))

	tool = mcp.NewTool(
		"estate_create",
		mcp.WithDescription("Создать словарную запись сословия; id генерируется сервером; результат — JSON созданной записи. variants/items/notes — списки текста (без ссылок на другие сущности, v1)"),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Каноническая форма")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, estateCreateHandler(estates))

	tool = mcp.NewTool(
		"estate_update",
		mcp.WithDescription("Изменить словарную запись сословия: обновляются переданные поля; при отсутствии аргумента в вызове соответствующее поле (variants/items/notes) сохраняет текущее значение, явный пустой список — очищает его; результат — JSON обновлённой записи. variants/items/notes, если переданы, заменяются целиком как текст — если у элемента раньше была ссылка на другую сущность (ref/type из estate_get), она будет потеряна: picker для ссылок ещё не реализован ни в вебе, ни в MCP (docs/data-model/entity-write.md §5)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Новая каноническая форма")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, estateUpdateHandler(estates))

	tool = mcp.NewTool(
		"estate_delete",
		mcp.WithDescription("Удалить словарную запись сословия. Необратимо. Если на неё есть строгие ссылки от других сущностей — ошибка тула (для Estate такое сегодня не создаётся, но общий механизм проверки один для всех сущностей)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, estateDeleteHandler(estates))
}

func estateListHandler(estates EstateService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := estates.ListEstates(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.EstatesFromModels(list))
	}
}

func estateSearchHandler(estates EstateService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := estates.SearchEstates(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.EstatesFromModels(list))
	}
}

func estateGetHandler(estates EstateService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn, err := estates.GetEstate(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.EstateFromModel(sn))
	}
}

func estateCreateHandler(estates EstateService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn := models.Estate{
			Canonical: req.GetString("canonical", ""),
			Variants:  textRefsFromStrings(req.GetStringSlice("variants", nil)),
			Items:     textRefsFromStrings(req.GetStringSlice("items", nil)),
			Notes:     textRefsFromStrings(req.GetStringSlice("notes", nil)),
		}

		created, err := estates.CreateEstate(ctx, sn)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.EstateFromModel(created))
	}
}

func estateUpdateHandler(estates EstateService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := estates.GetEstate(ctx, id)
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

		if err := estates.UpdateEstate(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.EstateFromModel(cur))
	}
}

func estateDeleteHandler(estates EstateService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := estates.DeleteEstate(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
