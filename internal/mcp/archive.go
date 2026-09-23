package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerArchiveTools регистрирует тулы для работы с архивами. system —
// система иерархии, только текстом (ссылка на сущность не допускается,
// models.Archive.Validate). repository_id — просто id (не объект TextRef, в
// отличие от system/parish/church у других сущностей): пустая строка — без
// хранилища; сценарий проверяет существование при непустом значении
// (archive_create/archive_update вернут ошибку тула на несуществующий id).
func registerArchiveTools(s *server.MCPServer, archives ArchiveService) {
	tool := mcp.NewTool(
		"archive_list",
		mcp.WithDescription("Список архивов в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, archiveListHandler(archives))

	tool = mcp.NewTool(
		"archive_search",
		mcp.WithDescription("Поиск архивов по началу названия; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, archiveSearchHandler(archives))

	tool = mcp.NewTool(
		"archive_get",
		mcp.WithDescription("Архив по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например AR-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, archiveGetHandler(archives))

	tool = mcp.NewTool(
		"archive_create",
		mcp.WithDescription("Создать архив; id генерируется сервером; результат — JSON созданной записи. Несуществующий repository_id — ошибка тула"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Название")),
		mcp.WithObject("system", mcp.Description("Система иерархии архива — только именем, без ссылки"), mcp.Properties(map[string]any{
			"text": map[string]any{"type": "string", "description": "Имя системы иерархии"},
		})),
		mcp.WithString("repository_id", mcp.Description("id хранилища (необязательно; пусто — без хранилища)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, archiveCreateHandler(archives))

	tool = mcp.NewTool(
		"archive_update",
		mcp.WithDescription("Изменить архив: полная замена name/system/repository_id/notes/private; результат — JSON обновлённой записи. Несуществующий repository_id — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Новое название")),
		mcp.WithObject("system", mcp.Description("Система иерархии — только именем"), mcp.Properties(map[string]any{
			"text": map[string]any{"type": "string", "description": "Имя системы иерархии"},
		})),
		mcp.WithString("repository_id", mcp.Description("id хранилища (необязательно)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, archiveUpdateHandler(archives))

	tool = mcp.NewTool(
		"archive_delete",
		mcp.WithDescription("Удалить архив. Необратимо. Если на него есть строгие ссылки от других сущностей — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, archiveDeleteHandler(archives))
}

func archiveListHandler(archives ArchiveService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := archives.ListArchives(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.ArchivesFromModels(list))
	}
}

func archiveSearchHandler(archives ArchiveService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := archives.SearchArchives(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.ArchivesFromModels(list))
	}
}

func archiveGetHandler(archives ArchiveService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a, err := archives.GetArchive(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveFromModel(a))
	}
}

func archiveCreateHandler(archives ArchiveService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		system, err := optionalTextRef(req.GetArguments(), "system")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		a := models.Archive{
			Name:         req.GetString("name", ""),
			System:       system,
			RepositoryID: models.ID(req.GetString("repository_id", "")),
			Notes:        textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Private:      req.GetBool("private", false),
		}

		created, err := archives.CreateArchive(ctx, a)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveFromModel(created))
	}
}

func archiveUpdateHandler(archives ArchiveService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := archives.GetArchive(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		system, err := optionalTextRef(req.GetArguments(), "system")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.Name = req.GetString("name", "")
		cur.System = system
		cur.RepositoryID = models.ID(req.GetString("repository_id", ""))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		cur.Private = req.GetBool("private", false)

		if err := archives.UpdateArchive(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveFromModel(cur))
	}
}

func archiveDeleteHandler(archives ArchiveService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := archives.DeleteArchive(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
