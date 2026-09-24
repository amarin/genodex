package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// divisionTypeList — перечень допустимых типов единицы для описаний тулов.
func divisionTypeList() string {
	names := make([]string, len(models.AdminDivisionTypes))
	for i, t := range models.AdminDivisionTypes {
		names[i] = string(t)
	}

	return strings.Join(names, ", ")
}

// registerDivisionTools регистрирует тулы для работы с единицами административного деления.
func registerDivisionTools(s *server.MCPServer, divisions DivisionService) {
	tool := mcp.NewTool(
		"division_list",
		mcp.WithDescription("Список единиц административного деления (губернии, уезды, волости, населённые "+
			"пункты) в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithString("kind", mcp.Description("Вид: settlement — только населённые пункты; пусто — без фильтра")),
		mcp.WithString("type", mcp.Description("Точный тип единицы: "+divisionTypeList()+"; пусто — без фильтра")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
		mcp.WithString("parent_id", mcp.Description("id родительской единицы; пусто — корень (весь список)")),
	)

	s.AddTool(tool, divisionListHandler(divisions))

	tool = mcp.NewTool(
		"division_search",
		mcp.WithDescription("Поиск единиц административного деления по началу названия (включая варианты "+
			"названий); результат — JSON-массив единиц. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия или варианта")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, divisionSearchHandler(divisions))

	tool = mcp.NewTool(
		"division_get",
		mcp.WithDescription("Единица административного деления по id; результат — JSON единицы. Неверный формат id или отсутствующая единица — ошибка тула; то же для единицы, ссылающейся на приватную цитату среди источников (sources[i].citation_id)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id единицы, например AD-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, divisionGetHandler(divisions))

	tool = mcp.NewTool(
		"division_create",
		mcp.WithDescription("Создать единицу административного деления; id генерируется сервером; результат — JSON созданной единицы. Неверные name/type или несуществующий parent_id — ошибка тула"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Название единицы")),
		mcp.WithString("type", mcp.Required(), mcp.Description(divisionTypeList())),
		mcp.WithString("parent_id", mcp.Description("id родительской единицы; пусто — корень")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
	)
	s.AddTool(tool, divisionCreateHandler(divisions))

	tool = mcp.NewTool(
		"division_update",
		mcp.WithDescription("Изменить единицу административного деления: результат — JSON обновлённой единицы. name/type — обязательны, заменяются всегда; parent_id — при отсутствии в вызове сохраняет текущего родителя, явная пустая строка делает единицу корнем; sources — при отсутствии в вызове текущие источники сохраняются, пустой массив — очищает их"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id единицы")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Новое название")),
		mcp.WithString("type", mcp.Required(), mcp.Description(divisionTypeList())),
		mcp.WithString("parent_id", mcp.Description("id нового родителя; пусто — корень")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
	)
	s.AddTool(tool, divisionUpdateHandler(divisions))

	tool = mcp.NewTool(
		"division_delete",
		mcp.WithDescription("Удалить единицу административного деления, если она не занята другими единицами; занятая — ошибка тула. Необратимо"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id единицы")),
	)
	s.AddTool(tool, divisionDeleteHandler(divisions))
}

// divisionListHandler возвращает обработчик тула division_list: разбирает
// аргументы, обращается к сценарию и возвращает JSON-массив контракта
// transport.AdminDivision. Ошибки тула сообщаются внутри CallToolResult через
// NewToolResultError.
func divisionListHandler(divisions DivisionService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q, err := divisionQueryFromRequest(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := divisions.ListDivisions(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		data, err := json.Marshal(transport.AdminDivisionsFromModels(list))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сериализовать список: %v", err)), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

// divisionQueryFromRequest собирает запрос из аргументов тула; отсутствующий
// аргумент — нулевое значение, неверный тип числа — ошибка.
func divisionQueryFromRequest(req mcp.CallToolRequest) (models.DivisionQuery, error) {
	q := models.DivisionQuery{
		Kind:     models.DivisionKind(req.GetString("kind", "")),
		Type:     models.AdminDivisionType(req.GetString("type", "")),
		ParentID: optionalParentID(req),
	}

	var err error

	if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
		return q, err
	}

	if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
		return q, err
	}

	return q, nil
}

// divisionSearchHandler — тул division_search: ищет единицы по началу названия
// (включая варианты); результат — JSON-массив transport.AdminDivision.
func divisionSearchHandler(divisions DivisionService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.DivisionSearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := divisions.SearchDivisions(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.AdminDivisionsFromModels(list))
	}
}

// optionalInt читает необязательный целочисленный аргумент.
func optionalInt(req mcp.CallToolRequest, name string) (int, error) {
	raw, ok := req.GetArguments()[name]
	if !ok || raw == nil { // нет аргумента или явный null — значение по умолчанию
		return 0, nil
	}

	if f, isFloat := raw.(float64); isFloat && f != math.Trunc(f) {
		return 0, fmt.Errorf("аргумент %s: ожидалось целое число", name)
	}

	n, err := req.RequireInt(name)
	if err != nil {
		return 0, fmt.Errorf("аргумент %s: ожидалось целое число", name)
	}

	return n, nil
}

// divisionGetHandler — тул division_get: читает единицу по id. Валидация
// формата id и проверка существования — в сценарии; его ошибки — ошибки тула.
func divisionGetHandler(divisions DivisionService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		d, err := divisions.GetDivision(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить единицу: %v", err)), nil
		}

		return toolJSONResult(transport.AdminDivisionFromModel(d))
	}
}

// divisionCreateHandler — тул division_create: собирает модель из аргументов
// (пустой parent_id — корень) и отдаёт созданную единицу.
func divisionCreateHandler(divisions DivisionService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		row := transport.AdminDivisionCreate{
			Name:     req.GetString("name", ""),
			Type:     models.AdminDivisionType(req.GetString("type", "")),
			ParentID: optionalParentID(req),
		}

		m := row.Model()
		m.Sources = sources

		created, err := divisions.CreateDivision(ctx, m)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать единицу: %v", err)), nil
		}

		return toolJSONResult(transport.AdminDivisionFromModel(created))
	}
}

// divisionUpdateHandler — тул division_update: name/type заменяются всегда
// (обязательны); parent_id при отсутствии в вызове сохраняет текущего
// родителя, явная пустая строка делает единицу корнем; sources при
// отсутствии в вызове сохраняет текущие источники, явный пустой массив —
// очищает их; остальные поля берутся из актуальной версии сценария.
func divisionUpdateHandler(divisions DivisionService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := divisions.GetDivision(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		row := transport.AdminDivisionUpdate{
			Name: req.GetString("name", ""),
			Type: models.AdminDivisionType(req.GetString("type", "")),
		}
		cur.Name = row.Name
		cur.Type = row.Type

		if raw, ok := args["parent_id"]; ok && raw != nil {
			row.ParentID = optionalParentID(req)
			cur.ParentID = row.ParentID
		}

		if raw, ok := args["sources"]; ok && raw != nil {
			sources, err := optionalSourceLinks(args, "sources")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Sources = sources
		}

		if err := divisions.UpdateDivision(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.AdminDivisionFromModel(cur))
	}
}

// divisionDeleteHandler — тул division_delete: удаляет единицу; занятая —
// ошибка тула с текстом ошибки сценария.
func divisionDeleteHandler(divisions DivisionService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := divisions.DeleteDivision(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить единицу: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("деление %q удалено", id)), nil
	}
}

// toolJSONResult сериализует значение в JSON-текст результата тула.
func toolJSONResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("не удалось сериализовать: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

// optionalParentID читает необязательный parent_id; пустая строка (в т.ч. явный
// null) — корень (nil-указатель).
func optionalParentID(req mcp.CallToolRequest) *models.ID {
	raw := req.GetString("parent_id", "")
	if raw == "" {
		return nil
	}

	id := models.ID(raw)

	return &id
}
