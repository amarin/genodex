package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerPersonTools регистрирует тулы для работы с персонами — ядром
// графа генеалогии. names — массив объектов вида PersonName (см.
// personNameObjectProperties, internal/mcp/object_args.go): вид имени + три
// мягкие ссылки на словари (surname/given/patronymic — существование НЕ
// проверяется, тот же принцип, что и у любого другого TextRef в программе)
// + служебные части + период. estates/titles/nicknames/notes — только
// текстом (v1, docs/data-model/entity-write.md §4). ВАЖНО: person_update
// заменяет estates/titles/nicknames/notes целиком текстом — существующие
// ref/type будут потеряны при любом обновлении через MCP, пока не появится
// picker; names и sources, наоборот, при отсутствии в вызове сохраняют
// текущее значение (см. personUpdateHandler).
func registerPersonTools(s *server.MCPServer, people PersonService) {
	tool := mcp.NewTool(
		"person_list",
		mcp.WithDescription("Список персон в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, personListHandler(people))

	tool = mcp.NewTool(
		"person_search",
		mcp.WithDescription("Поиск персон по началу фамилии, имени или отчества из ЛЮБОГО из имён персоны (не только основного); результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало фамилии, имени или отчества")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, personSearchHandler(people))

	tool = mcp.NewTool(
		"person_get",
		mcp.WithDescription("Персона по id; результат — JSON записи. Неверный формат id или отсутствующая/приватная (для не-владельца) запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например I-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, personGetHandler(people))

	tool = mcp.NewTool(
		"person_create",
		mcp.WithDescription("Создать персону; id генерируется сервером; результат — JSON созданной записи. Все поля, кроме id, необязательны"),
		mcp.WithString("gender", mcp.Description("Пол: male/female/unknown; пусто — не указан")),
		mcp.WithArray("names", mcp.Items(map[string]any{
			"type":       "object",
			"properties": personNameObjectProperties(),
		}), mcp.Description("Имена персоны (основное, при рождении, по браку…); каждое — хотя бы одна из частей surname/given/patronymic")),
		mcp.WithArray("estates", mcp.WithStringItems(), mcp.Description("Сословия (текстом; мягкая ссылка на словарь сословий, без проверки существования)")),
		mcp.WithArray("titles", mcp.WithStringItems(), mcp.Description("Титулы (текстом; мягкая ссылка на словарь титулов, без проверки существования)")),
		mcp.WithArray("nicknames", mcp.WithStringItems(), mcp.Description("Прозвища")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, personCreateHandler(people))

	tool = mcp.NewTool(
		"person_update",
		mcp.WithDescription("Изменить персону: полная замена gender/estates/titles/nicknames/notes/private; результат — JSON обновлённой записи; names и sources — при отсутствии в вызове текущее значение сохраняется, пустой массив — очищает его"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("gender", mcp.Description("Пол: male/female/unknown; пусто — не указан")),
		mcp.WithArray("names", mcp.Items(map[string]any{
			"type":       "object",
			"properties": personNameObjectProperties(),
		}), mcp.Description("Имена персоны; при отсутствии в вызове текущие имена сохраняются, пустой массив — очищает их")),
		mcp.WithArray("estates", mcp.WithStringItems(), mcp.Description("Сословия")),
		mcp.WithArray("titles", mcp.WithStringItems(), mcp.Description("Титулы")),
		mcp.WithArray("nicknames", mcp.WithStringItems(), mcp.Description("Прозвища")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, personUpdateHandler(people))

	tool = mcp.NewTool(
		"person_delete",
		mcp.WithDescription("Удалить персону. Необратимо. Если на неё есть строгие ссылки от других сущностей — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, personDeleteHandler(people))
}

func personListHandler(people PersonService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := people.ListPeople(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.PeopleFromModels(list))
	}
}

func personSearchHandler(people PersonService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := people.SearchPeople(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.PeopleFromModels(list))
	}
}

func personGetHandler(people PersonService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		p, err := people.GetPerson(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.PersonFromModel(p))
	}
}

func personCreateHandler(people PersonService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		names, err := optionalPersonNames(req.GetArguments(), "names")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		p := models.Person{
			Gender:    models.PersonGender(req.GetString("gender", "")),
			Names:     names,
			Estates:   textRefsFromStrings(req.GetStringSlice("estates", nil)),
			Titles:    textRefsFromStrings(req.GetStringSlice("titles", nil)),
			Nicknames: textRefsFromStrings(req.GetStringSlice("nicknames", nil)),
			Notes:     textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Sources:   sources,
			Private:   req.GetBool("private", false),
		}

		created, err := people.CreatePerson(ctx, p)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.PersonFromModel(created))
	}
}

func personUpdateHandler(people PersonService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := people.GetPerson(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		cur.Gender = models.PersonGender(req.GetString("gender", ""))
		cur.Estates = textRefsFromStrings(req.GetStringSlice("estates", nil))
		cur.Titles = textRefsFromStrings(req.GetStringSlice("titles", nil))
		cur.Nicknames = textRefsFromStrings(req.GetStringSlice("nicknames", nil))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		cur.Private = req.GetBool("private", false)

		if raw, ok := req.GetArguments()["names"]; ok && raw != nil {
			names, err := optionalPersonNames(req.GetArguments(), "names")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Names = names
		}

		if raw, ok := req.GetArguments()["sources"]; ok && raw != nil {
			sources, err := optionalSourceLinks(req.GetArguments(), "sources")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Sources = sources
		}

		if err := people.UpdatePerson(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.PersonFromModel(cur))
	}
}

func personDeleteHandler(people PersonService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := people.DeletePerson(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
