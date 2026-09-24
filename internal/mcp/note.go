package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerNoteTools регистрирует тулы для работы с заметками. parent_id —
// просто id родительской заметки (не объект, в отличие от church/parish's
// одиночного TextRef — self-ref строгая ссылка, как archive's
// repository_id): пустая строка — без родителя. note_create/note_update
// проверяют существование родителя и отсутствие циклов (по образцу
// division). Приватная заметка недоступна вызывающему без полного доступа —
// note_get отдаёт ошибку тула, как и List/Search её не покажут.
func registerNoteTools(s *server.MCPServer, notes NoteService) {
	tool := mcp.NewTool(
		"note_list",
		mcp.WithDescription("Список заметок в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, noteListHandler(notes))

	tool = mcp.NewTool(
		"note_search",
		mcp.WithDescription("Поиск заметок по началу заголовка; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало заголовка")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, noteSearchHandler(notes))

	tool = mcp.NewTool(
		"note_get",
		mcp.WithDescription("Заметка по id; результат — JSON записи. Неверный формат id, отсутствующая или приватная (без полного доступа) запись — ошибка тула; то же для заметки, ссылающейся на приватную цитату среди источников (sources[i].citation_id)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например N-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, noteGetHandler(notes))

	tool = mcp.NewTool(
		"note_create",
		mcp.WithDescription("Создать заметку; id генерируется сервером; результат — JSON созданной записи. Нужен заголовок или текст. Несуществующий parent_id — ошибка тула"),
		mcp.WithString("kind", mcp.Required(), mcp.Description("Вид заметки: открытый список (note/article/book/chapter — типовые значения, допустимы и другие), формат [a-z][a-z0-9_-]*")),
		mcp.WithString("title", mcp.Description("Заголовок")),
		mcp.WithString("text", mcp.Description("Текст (markdown)")),
		mcp.WithString("parent_id", mcp.Description("id родительской заметки (необязательно; пусто — без родителя)")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, noteCreateHandler(notes))

	tool = mcp.NewTool(
		"note_update",
		mcp.WithDescription("Изменить заметку: обновляются переданные поля; при отсутствии аргумента в вызове соответствующее поле (title/text/parent_id/private/sources) сохраняет текущее значение, явное пустое значение/пустой массив — очищает его; результат — JSON обновлённой записи. Несуществующий parent_id или цикл в цепочке родителей — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("kind", mcp.Required(), mcp.Description("Вид заметки")),
		mcp.WithString("title", mcp.Description("Заголовок")),
		mcp.WithString("text", mcp.Description("Текст (markdown)")),
		mcp.WithString("parent_id", mcp.Description("id родительской заметки (необязательно)")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, noteUpdateHandler(notes))

	tool = mcp.NewTool(
		"note_delete",
		mcp.WithDescription("Удалить заметку. Необратимо. Если есть дочерние заметки или другие строгие ссылки — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, noteDeleteHandler(notes))
}

func noteListHandler(notes NoteService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := notes.ListNotes(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.NotesFromModels(list))
	}
}

func noteSearchHandler(notes NoteService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := notes.SearchNotes(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.NotesFromModels(list))
	}
}

func noteGetHandler(notes NoteService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		n, err := notes.GetNote(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.NoteFromModel(n))
	}
}

func noteCreateHandler(notes NoteService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		n := models.Note{
			Kind:    models.NoteKind(req.GetString("kind", "")),
			Title:   req.GetString("title", ""),
			Text:    req.GetString("text", ""),
			Sources: sources,
			Private: req.GetBool("private", false),
		}

		if pid := req.GetString("parent_id", ""); pid != "" {
			id := models.ID(pid)
			n.ParentID = &id
		}

		created, err := notes.CreateNote(ctx, n)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.NoteFromModel(created))
	}
}

func noteUpdateHandler(notes NoteService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := notes.GetNote(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		cur.Kind = models.NoteKind(req.GetString("kind", ""))

		if raw, ok := args["title"]; ok && raw != nil {
			cur.Title = req.GetString("title", "")
		}

		if raw, ok := args["text"]; ok && raw != nil {
			cur.Text = req.GetString("text", "")
		}

		if raw, ok := args["private"]; ok && raw != nil {
			cur.Private = req.GetBool("private", false)
		}

		if raw, ok := args["parent_id"]; ok && raw != nil {
			if pid := req.GetString("parent_id", ""); pid != "" {
				pidID := models.ID(pid)
				cur.ParentID = &pidID
			} else {
				cur.ParentID = nil
			}
		}

		if raw, ok := args["sources"]; ok && raw != nil {
			sources, err := optionalSourceLinks(args, "sources")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Sources = sources
		}

		if err := notes.UpdateNote(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.NoteFromModel(cur))
	}
}

func noteDeleteHandler(notes NoteService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := notes.DeleteNote(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
