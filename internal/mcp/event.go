package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerEventTools регистрирует тулы для работы с событиями. event_list
// принимает необязательный person_id (совпадает с любым из
// Participants[i].PersonID). event_search ЕСТЬ (в отличие от Relation/
// Residence) — ищет ТОЛЬКО по началу текста места (place), единственное
// индексируемое поле события (internal/store/sqlstore/records.go:SaveEvent).
// type — REQUIRED и в event_update (безусловная полная замена, как
// relation_update's kind). date/place — одиночные объектные поля
// (*FactDate/*PlaceRef): presence-ONLY guard (отсутствие ключа сохраняет
// текущее значение, явный null очищает — {} не проходит валидацию).
// participants — массив объектов, PersonID внутри — ПЕРВАЯ строгая
// (проверяемая на существование) ссылка внутри array-of-objects MCP-
// аргумента в программе (в отличие от sources/names, чьи ссылки мягкие или
// уже установлены) — обычный preserve-on-omit array-of-objects guard, как
// sources/notes.
func registerEventTools(s *server.MCPServer, events EventService) {
	tool := mcp.NewTool(
		"event_list",
		mcp.WithDescription("Список событий в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithString("person_id", mcp.Description("Необязательный фильтр: только события, где эта персона — участник")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, eventListHandler(events))

	tool = mcp.NewTool(
		"event_search",
		mcp.WithDescription("Поиск событий ТОЛЬКО по началу текста места (place) — единственное индексируемое поле события (type/date/участники поиском не охвачены); результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало текста места")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, eventSearchHandler(events))

	tool = mcp.NewTool(
		"event_get",
		mcp.WithDescription("Событие по id; результат — JSON записи. Неверный формат id или отсутствующая/приватная (для не-владельца) запись — ошибка тула; то же для события, ссылающегося на приватного участника (participants[i].person_id) — и в event_search; то же для события, ссылающегося на приватную цитату среди источников (sources[i].citation_id) — и в event_search"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например E-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, eventGetHandler(events))

	tool = mcp.NewTool(
		"event_create",
		mcp.WithDescription("Создать событие; id генерируется сервером; результат — JSON созданной записи. Несуществующий participants[i].person_id — ошибка тула; place — мягкая ссылка, НИКОГДА не проверяется на существование"),
		mcp.WithString("type", mcp.Required(), mcp.Description("Вид события (открытый набор: birth/death/marriage/burial/confession/census и др.)")),
		mcp.WithObject("date", mcp.Description("Дата события"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("place", mcp.Description("Место (текст или ссылка на административное деление/церковь/приход; ref/type сохраняются, если переданы; существование НЕ проверяется)"), mcp.Properties(placeRefObjectProperties())),
		mcp.WithArray("participants", mcp.Items(map[string]any{
			"type":       "object",
			"properties": eventParticipantObjectProperties(),
			"required":   []string{"person_id", "role"},
		}), mcp.Description("Участники события (person_id проверяется на существование)")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, eventCreateHandler(events))

	tool = mcp.NewTool(
		"event_update",
		mcp.WithDescription("Изменить событие: type заменяется безусловно при каждом вызове; participants/sources/notes/private — при отсутствии аргумента сохраняют текущее значение, явное пустое значение/пустой список — очищает; date/place — одиночные объектные поля: отсутствие ключа сохраняет текущее значение, явный null — очищает (пустой объект {} для очистки не подходит — не проходит валидацию); результат — JSON обновлённой записи"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("type", mcp.Required(), mcp.Description("Вид события")),
		mcp.WithObject("date", mcp.Description("Дата события"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("place", mcp.Description("Место (текст или ссылка); существование НЕ проверяется"), mcp.Properties(placeRefObjectProperties())),
		mcp.WithArray("participants", mcp.Items(map[string]any{
			"type":       "object",
			"properties": eventParticipantObjectProperties(),
			"required":   []string{"person_id", "role"},
		}), mcp.Description("Участники события; при отсутствии в вызове текущий список сохраняется, пустой массив — очищает его")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, eventUpdateHandler(events))

	tool = mcp.NewTool(
		"event_delete",
		mcp.WithDescription("Удалить событие. Необратимо. Если на него есть строгие ссылки — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, eventDeleteHandler(events))
}

func eventListHandler(events EventService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.EventQuery{}

		if pid := req.GetString("person_id", ""); pid != "" {
			id := models.ID(pid)
			q.PersonID = &id
		}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := events.ListEvents(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.EventsFromModels(list))
	}
}

func eventSearchHandler(events EventService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := events.SearchEvents(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.EventsFromModels(list))
	}
}

func eventGetHandler(events EventService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		e, err := events.GetEvent(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.EventFromModel(e))
	}
}

func eventCreateHandler(events EventService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		date, err := optionalFactDate(args, "date")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		place, err := optionalPlaceRef(args, "place")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		participants, err := optionalEventParticipants(args, "participants")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sources, err := optionalSourceLinks(args, "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		e := models.Event{
			Type:         models.EventType(req.GetString("type", "")),
			Date:         date,
			Place:        place,
			Participants: participants,
			Sources:      sources,
			Notes:        textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Private:      req.GetBool("private", false),
		}

		created, err := events.CreateEvent(ctx, e)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.EventFromModel(created))
	}
}

func eventUpdateHandler(events EventService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := events.GetEvent(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		cur.Type = models.EventType(req.GetString("type", ""))

		if _, ok := args["date"]; ok {
			date, err := optionalFactDate(args, "date")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Date = date
		}

		if _, ok := args["place"]; ok {
			place, err := optionalPlaceRef(args, "place")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Place = place
		}

		if raw, ok := args["participants"]; ok && raw != nil {
			participants, err := optionalEventParticipants(args, "participants")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Participants = participants
		}

		if raw, ok := args["sources"]; ok && raw != nil {
			sources, err := optionalSourceLinks(args, "sources")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Sources = sources
		}

		if raw, ok := args["notes"]; ok && raw != nil {
			cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		}

		if raw, ok := args["private"]; ok && raw != nil {
			cur.Private = req.GetBool("private", false)
		}

		if err := events.UpdateEvent(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.EventFromModel(cur))
	}
}

func eventDeleteHandler(events EventService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := events.DeleteEvent(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
