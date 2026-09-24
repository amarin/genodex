package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerResidenceTools регистрирует тулы для работы с проживаниями. Без
// residence_search — search_residences намеренно не заводится (индекс пуст,
// docs/data-model/entity-write.md §3.8); residence_list принимает
// необязательные person_id/place_id (пересекаются, если оба заданы).
// person_id/place_id — REQUIRED и в residence_update (безусловная полная
// замена, ядро того, что представляет собой запись — как у relation_update);
// since/until — presence-ONLY guard (см. registerRelationTools). note —
// единственная строка (не список), обычный preserve-on-omit скаляр.
func registerResidenceTools(s *server.MCPServer, residences ResidenceService) {
	tool := mcp.NewTool(
		"residence_list",
		mcp.WithDescription("Список проживаний в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithString("person_id", mcp.Description("Необязательный фильтр по персоне")),
		mcp.WithString("place_id", mcp.Description("Необязательный фильтр по месту")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, residenceListHandler(residences))

	tool = mcp.NewTool(
		"residence_get",
		mcp.WithDescription("Проживание по id; результат — JSON записи. Неверный формат id или отсутствующая/приватная (для не-владельца) запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например RS-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, residenceGetHandler(residences))

	tool = mcp.NewTool(
		"residence_create",
		mcp.WithDescription("Создать проживание; id генерируется сервером; результат — JSON созданной записи. Несуществующие person_id/place_id — ошибка тула"),
		mcp.WithString("person_id", mcp.Required(), mcp.Description("id персоны")),
		mcp.WithString("place_id", mcp.Required(), mcp.Description("id места (административное деление)")),
		mcp.WithObject("since", mcp.Description("Начало проживания"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец проживания"), mcp.Properties(factDateObjectProperties())),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithString("note", mcp.Description("Заметка")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, residenceCreateHandler(residences))

	tool = mcp.NewTool(
		"residence_update",
		mcp.WithDescription("Изменить проживание: person_id/place_id заменяются безусловно при каждом вызове; sources/note/private — при отсутствии аргумента сохраняют текущее значение, явное пустое значение/пустой список — очищает; since/until — одиночные объектные поля: отсутствие ключа сохраняет текущее значение, явный null — очищает; результат — JSON обновлённой записи"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("person_id", mcp.Required(), mcp.Description("id персоны")),
		mcp.WithString("place_id", mcp.Required(), mcp.Description("id места")),
		mcp.WithObject("since", mcp.Description("Начало проживания"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец проживания"), mcp.Properties(factDateObjectProperties())),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithString("note", mcp.Description("Заметка")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, residenceUpdateHandler(residences))

	tool = mcp.NewTool(
		"residence_delete",
		mcp.WithDescription("Удалить проживание. Необратимо. Если на него есть строгие ссылки — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, residenceDeleteHandler(residences))
}

func residenceListHandler(residences ResidenceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.ResidenceQuery{}

		if pid := req.GetString("person_id", ""); pid != "" {
			id := models.ID(pid)
			q.PersonID = &id
		}

		if plid := req.GetString("place_id", ""); plid != "" {
			id := models.ID(plid)
			q.PlaceID = &id
		}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := residences.ListResidences(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.ResidencesFromModels(list))
	}
}

func residenceGetHandler(residences ResidenceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		r, err := residences.GetResidence(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.ResidenceFromModel(r))
	}
}

func residenceCreateHandler(residences ResidenceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

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

		r := models.Residence{
			PersonID: models.ID(req.GetString("person_id", "")),
			PlaceID:  models.ID(req.GetString("place_id", "")),
			Since:    since,
			Until:    until,
			Sources:  sources,
			Note:     req.GetString("note", ""),
			Private:  req.GetBool("private", false),
		}

		created, err := residences.CreateResidence(ctx, r)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.ResidenceFromModel(created))
	}
}

func residenceUpdateHandler(residences ResidenceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := residences.GetResidence(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		cur.PersonID = models.ID(req.GetString("person_id", ""))
		cur.PlaceID = models.ID(req.GetString("place_id", ""))

		if _, ok := args["since"]; ok {
			since, err := optionalFactDate(args, "since")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Since = since
		}

		if _, ok := args["until"]; ok {
			until, err := optionalFactDate(args, "until")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Until = until
		}

		if raw, ok := args["sources"]; ok && raw != nil {
			sources, err := optionalSourceLinks(args, "sources")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Sources = sources
		}

		if raw, ok := args["note"]; ok && raw != nil {
			cur.Note = req.GetString("note", "")
		}

		if raw, ok := args["private"]; ok && raw != nil {
			cur.Private = req.GetBool("private", false)
		}

		if err := residences.UpdateResidence(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.ResidenceFromModel(cur))
	}
}

func residenceDeleteHandler(residences ResidenceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := residences.DeleteResidence(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
