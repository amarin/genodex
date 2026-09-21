package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerDivisionTools регистрирует тулы для работы с единицами административного деления.
func registerDivisionTools(s *server.MCPServer, divisions DivisionService) {
	tool := mcp.NewTool(
		"division_list",
		mcp.WithDescription("Список единиц административного деления (губернии, уезды, волости, населённые "+
			"пункты) в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithString("kind", mcp.Description("Вид: settlement — только населённые пункты; пусто — без фильтра")),
		mcp.WithString("type", mcp.Description("Точный тип единицы: governorate, district, volost, gorod, selo, "+
			"derevnya, hutor, pogost, stanitsa, mestechko, other; пусто — без фильтра")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)

	s.AddTool(tool, divisionListHandler(divisions))
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

		list, err := divisions.ListDivisions(ctx, q)
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
		Kind: models.DivisionKind(req.GetString("kind", "")),
		Type: models.AdminDivisionType(req.GetString("type", "")),
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

// optionalInt читает необязательный целочисленный аргумент.
func optionalInt(req mcp.CallToolRequest, name string) (int, error) {
	if _, ok := req.GetArguments()[name]; !ok {
		return 0, nil
	}

	n, err := req.RequireInt(name)
	if err != nil {
		return 0, fmt.Errorf("аргумент %s: ожидалось целое число", name)
	}

	return n, nil
}
