package httpapi

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// NewHandler возвращает http.Handler с маршрутами /api (без auth-
// оборачивания) — используется юнит-тестами этого пакета напрямую.
// Реальное приложение монтирует NewAPIHandler (api.go). Deps.Auth/TrustProxy
// не используются (без auth-обёртки), Deps.Surnames может быть nil, если
// тесту нужны только маршруты делений.
func NewHandler(deps Deps) http.Handler {
	mux := http.NewServeMux()
	registerDivisionRoutes(mux, deps.Divisions, deps.DocsFS)

	if deps.Surnames != nil {
		registerSurnameRoutes(mux, deps.Surnames)
	}

	if deps.Patronymics != nil {
		registerPatronymicRoutes(mux, deps.Patronymics)
	}

	if deps.Estates != nil {
		registerEstateRoutes(mux, deps.Estates)
	}

	if deps.Titles != nil {
		registerTitleRoutes(mux, deps.Titles)
	}

	if deps.GivenNames != nil {
		registerGivenNameRoutes(mux, deps.GivenNames)
	}

	if deps.Repositories != nil {
		registerRepositoryRoutes(mux, deps.Repositories)
	}

	if deps.Churches != nil {
		registerChurchRoutes(mux, deps.Churches)
	}

	if deps.Parishes != nil {
		registerParishRoutes(mux, deps.Parishes)
	}

	if deps.Archives != nil {
		registerArchiveRoutes(mux, deps.Archives)
	}

	if deps.ArchiveNodes != nil {
		registerArchiveNodeRoutes(mux, deps.ArchiveNodes)
	}

	if deps.ArchiveDocs != nil {
		registerArchiveDocumentRoutes(mux, deps.ArchiveDocs)
	}

	if deps.Notes != nil {
		registerNoteRoutes(mux, deps.Notes)
	}

	if deps.Attachments != nil {
		registerAttachmentRoutes(mux, deps.Attachments)
	}

	if deps.Sources != nil {
		registerSourceRoutes(mux, deps.Sources)
	}

	if deps.Citations != nil {
		registerCitationRoutes(mux, deps.Citations)
	}

	if deps.Families != nil {
		registerFamilyRoutes(mux, deps.Families)
	}

	if deps.People != nil {
		registerPersonRoutes(mux, deps.People)
	}

	if deps.Relations != nil {
		registerRelationRoutes(mux, deps.Relations)
	}

	if deps.Residences != nil {
		registerResidenceRoutes(mux, deps.Residences)
	}

	if deps.Events != nil {
		registerEventRoutes(mux, deps.Events)
	}

	return mux
}

// registerDivisionRoutes регистрирует маршруты /api/admin-divisions,
// /api/admin-division-types,
// /api/docs, /api/health на переданном mux — общий код NewHandler и
// NewAPIHandler.
func registerDivisionRoutes(mux *http.ServeMux, divisions DivisionService, docsFS fs.FS) {
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/admin-divisions", handleDivisionList(divisions))
	mux.HandleFunc("GET /api/admin-divisions/search", handleDivisionSearch(divisions))
	mux.HandleFunc("GET /api/admin-divisions/{id}", handleDivisionGet(divisions))
	mux.HandleFunc("POST /api/admin-divisions", handleDivisionCreate(divisions))
	mux.HandleFunc("PUT /api/admin-divisions/{id}", handleDivisionUpdate(divisions))
	mux.HandleFunc("DELETE /api/admin-divisions/{id}", handleDivisionDelete(divisions))
	mux.HandleFunc("GET /api/admin-division-types", handleDivisionTypeList(divisions))
	mux.HandleFunc("GET /api/docs", handleDocList(docsFS))
	mux.HandleFunc("GET /api/docs/{path}", handleDocContent(docsFS))
}

// registerSurnameRoutes регистрирует маршруты /api/surnames на переданном mux.
func registerSurnameRoutes(mux *http.ServeMux, surnames SurnameService) {
	mux.HandleFunc("GET /api/surnames", handleSurnameList(surnames))
	mux.HandleFunc("GET /api/surnames/search", handleSurnameSearch(surnames))
	mux.HandleFunc("GET /api/surnames/{id}", handleSurnameGet(surnames))
	mux.HandleFunc("POST /api/surnames", handleSurnameCreate(surnames))
	mux.HandleFunc("PUT /api/surnames/{id}", handleSurnameUpdate(surnames))
	mux.HandleFunc("DELETE /api/surnames/{id}", handleSurnameDelete(surnames))
}

// registerPatronymicRoutes регистрирует маршруты /api/patronymics на переданном mux.
func registerPatronymicRoutes(mux *http.ServeMux, patronymics PatronymicService) {
	mux.HandleFunc("GET /api/patronymics", handlePatronymicList(patronymics))
	mux.HandleFunc("GET /api/patronymics/search", handlePatronymicSearch(patronymics))
	mux.HandleFunc("GET /api/patronymics/{id}", handlePatronymicGet(patronymics))
	mux.HandleFunc("POST /api/patronymics", handlePatronymicCreate(patronymics))
	mux.HandleFunc("PUT /api/patronymics/{id}", handlePatronymicUpdate(patronymics))
	mux.HandleFunc("DELETE /api/patronymics/{id}", handlePatronymicDelete(patronymics))
}

// registerEstateRoutes регистрирует маршруты /api/estates на переданном mux.
func registerEstateRoutes(mux *http.ServeMux, estates EstateService) {
	mux.HandleFunc("GET /api/estates", handleEstateList(estates))
	mux.HandleFunc("GET /api/estates/search", handleEstateSearch(estates))
	mux.HandleFunc("GET /api/estates/{id}", handleEstateGet(estates))
	mux.HandleFunc("POST /api/estates", handleEstateCreate(estates))
	mux.HandleFunc("PUT /api/estates/{id}", handleEstateUpdate(estates))
	mux.HandleFunc("DELETE /api/estates/{id}", handleEstateDelete(estates))
}

// registerTitleRoutes регистрирует маршруты /api/titles на переданном mux.
func registerTitleRoutes(mux *http.ServeMux, titles TitleService) {
	mux.HandleFunc("GET /api/titles", handleTitleList(titles))
	mux.HandleFunc("GET /api/titles/search", handleTitleSearch(titles))
	mux.HandleFunc("GET /api/titles/{id}", handleTitleGet(titles))
	mux.HandleFunc("POST /api/titles", handleTitleCreate(titles))
	mux.HandleFunc("PUT /api/titles/{id}", handleTitleUpdate(titles))
	mux.HandleFunc("DELETE /api/titles/{id}", handleTitleDelete(titles))
}

// registerGivenNameRoutes регистрирует маршруты /api/given-names на переданном mux.
func registerGivenNameRoutes(mux *http.ServeMux, givenNames GivenNameService) {
	mux.HandleFunc("GET /api/given-names", handleGivenNameList(givenNames))
	mux.HandleFunc("GET /api/given-names/search", handleGivenNameSearch(givenNames))
	mux.HandleFunc("GET /api/given-names/{id}", handleGivenNameGet(givenNames))
	mux.HandleFunc("POST /api/given-names", handleGivenNameCreate(givenNames))
	mux.HandleFunc("PUT /api/given-names/{id}", handleGivenNameUpdate(givenNames))
	mux.HandleFunc("DELETE /api/given-names/{id}", handleGivenNameDelete(givenNames))
}

// registerRepositoryRoutes регистрирует маршруты /api/repositories на переданном mux.
func registerRepositoryRoutes(mux *http.ServeMux, repositories RepositoryService) {
	mux.HandleFunc("GET /api/repositories", handleRepositoryList(repositories))
	mux.HandleFunc("GET /api/repositories/search", handleRepositorySearch(repositories))
	mux.HandleFunc("GET /api/repositories/{id}", handleRepositoryGet(repositories))
	mux.HandleFunc("POST /api/repositories", handleRepositoryCreate(repositories))
	mux.HandleFunc("PUT /api/repositories/{id}", handleRepositoryUpdate(repositories))
	mux.HandleFunc("DELETE /api/repositories/{id}", handleRepositoryDelete(repositories))
}

// registerChurchRoutes регистрирует маршруты /api/churches на переданном mux.
func registerChurchRoutes(mux *http.ServeMux, churches ChurchService) {
	mux.HandleFunc("GET /api/churches", handleChurchList(churches))
	mux.HandleFunc("GET /api/churches/search", handleChurchSearch(churches))
	mux.HandleFunc("GET /api/churches/{id}", handleChurchGet(churches))
	mux.HandleFunc("POST /api/churches", handleChurchCreate(churches))
	mux.HandleFunc("PUT /api/churches/{id}", handleChurchUpdate(churches))
	mux.HandleFunc("DELETE /api/churches/{id}", handleChurchDelete(churches))
}

// registerParishRoutes регистрирует маршруты /api/parishes на переданном mux.
func registerParishRoutes(mux *http.ServeMux, parishes ParishService) {
	mux.HandleFunc("GET /api/parishes", handleParishList(parishes))
	mux.HandleFunc("GET /api/parishes/search", handleParishSearch(parishes))
	mux.HandleFunc("GET /api/parishes/{id}", handleParishGet(parishes))
	mux.HandleFunc("POST /api/parishes", handleParishCreate(parishes))
	mux.HandleFunc("PUT /api/parishes/{id}", handleParishUpdate(parishes))
	mux.HandleFunc("DELETE /api/parishes/{id}", handleParishDelete(parishes))
}

// registerArchiveRoutes регистрирует маршруты /api/archives на переданном mux.
func registerArchiveRoutes(mux *http.ServeMux, archives ArchiveService) {
	mux.HandleFunc("GET /api/archives", handleArchiveList(archives))
	mux.HandleFunc("GET /api/archives/search", handleArchiveSearch(archives))
	mux.HandleFunc("GET /api/archives/{id}", handleArchiveGet(archives))
	mux.HandleFunc("POST /api/archives", handleArchiveCreate(archives))
	mux.HandleFunc("PUT /api/archives/{id}", handleArchiveUpdate(archives))
	mux.HandleFunc("DELETE /api/archives/{id}", handleArchiveDelete(archives))
}

// registerArchiveNodeRoutes регистрирует маршруты /api/archive-nodes на
// переданном mux. archive_id обязателен для списка (см. parseArchiveNodeQuery).
func registerArchiveNodeRoutes(mux *http.ServeMux, archiveNodes ArchiveNodeService) {
	mux.HandleFunc("GET /api/archive-nodes", handleArchiveNodeList(archiveNodes))
	mux.HandleFunc("GET /api/archive-nodes/search", handleArchiveNodeSearch(archiveNodes))
	mux.HandleFunc("GET /api/archive-nodes/{id}", handleArchiveNodeGet(archiveNodes))
	mux.HandleFunc("POST /api/archive-nodes", handleArchiveNodeCreate(archiveNodes))
	mux.HandleFunc("PUT /api/archive-nodes/{id}", handleArchiveNodeUpdate(archiveNodes))
	mux.HandleFunc("DELETE /api/archive-nodes/{id}", handleArchiveNodeDelete(archiveNodes))
}

// registerArchiveDocumentRoutes регистрирует маршруты /api/archive-documents
// на переданном mux.
func registerArchiveDocumentRoutes(mux *http.ServeMux, archiveDocuments ArchiveDocumentService) {
	mux.HandleFunc("GET /api/archive-documents", handleArchiveDocumentList(archiveDocuments))
	mux.HandleFunc("GET /api/archive-documents/search", handleArchiveDocumentSearch(archiveDocuments))
	mux.HandleFunc("GET /api/archive-documents/{id}", handleArchiveDocumentGet(archiveDocuments))
	mux.HandleFunc("POST /api/archive-documents", handleArchiveDocumentCreate(archiveDocuments))
	mux.HandleFunc("PUT /api/archive-documents/{id}", handleArchiveDocumentUpdate(archiveDocuments))
	mux.HandleFunc("DELETE /api/archive-documents/{id}", handleArchiveDocumentDelete(archiveDocuments))
}

// registerNoteRoutes регистрирует маршруты /api/notes на переданном mux.
func registerNoteRoutes(mux *http.ServeMux, notes NoteService) {
	mux.HandleFunc("GET /api/notes", handleNoteList(notes))
	mux.HandleFunc("GET /api/notes/search", handleNoteSearch(notes))
	mux.HandleFunc("GET /api/notes/{id}", handleNoteGet(notes))
	mux.HandleFunc("POST /api/notes", handleNoteCreate(notes))
	mux.HandleFunc("PUT /api/notes/{id}", handleNoteUpdate(notes))
	mux.HandleFunc("DELETE /api/notes/{id}", handleNoteDelete(notes))
}

// registerAttachmentRoutes регистрирует маршруты /api/attachments на переданном mux.
func registerAttachmentRoutes(mux *http.ServeMux, attachments AttachmentService) {
	mux.HandleFunc("GET /api/attachments", handleAttachmentList(attachments))
	mux.HandleFunc("GET /api/attachments/search", handleAttachmentSearch(attachments))
	mux.HandleFunc("GET /api/attachments/{id}", handleAttachmentGet(attachments))
	mux.HandleFunc("POST /api/attachments", handleAttachmentCreate(attachments))
	mux.HandleFunc("PUT /api/attachments/{id}", handleAttachmentUpdate(attachments))
	mux.HandleFunc("DELETE /api/attachments/{id}", handleAttachmentDelete(attachments))
}

// registerSourceRoutes регистрирует маршруты /api/sources на переданном mux.
func registerSourceRoutes(mux *http.ServeMux, sources SourceService) {
	mux.HandleFunc("GET /api/sources", handleSourceList(sources))
	mux.HandleFunc("GET /api/sources/search", handleSourceSearch(sources))
	mux.HandleFunc("GET /api/sources/{id}", handleSourceGet(sources))
	mux.HandleFunc("POST /api/sources", handleSourceCreate(sources))
	mux.HandleFunc("PUT /api/sources/{id}", handleSourceUpdate(sources))
	mux.HandleFunc("DELETE /api/sources/{id}", handleSourceDelete(sources))
}

// registerCitationRoutes регистрирует маршруты /api/citations на переданном mux.
func registerCitationRoutes(mux *http.ServeMux, citations CitationService) {
	mux.HandleFunc("GET /api/citations", handleCitationList(citations))
	mux.HandleFunc("GET /api/citations/search", handleCitationSearch(citations))
	mux.HandleFunc("GET /api/citations/{id}", handleCitationGet(citations))
	mux.HandleFunc("POST /api/citations", handleCitationCreate(citations))
	mux.HandleFunc("PUT /api/citations/{id}", handleCitationUpdate(citations))
	mux.HandleFunc("DELETE /api/citations/{id}", handleCitationDelete(citations))
}

// registerFamilyRoutes регистрирует маршруты /api/families на переданном mux.
func registerFamilyRoutes(mux *http.ServeMux, families FamilyService) {
	mux.HandleFunc("GET /api/families", handleFamilyList(families))
	mux.HandleFunc("GET /api/families/search", handleFamilySearch(families))
	mux.HandleFunc("GET /api/families/{id}", handleFamilyGet(families))
	mux.HandleFunc("POST /api/families", handleFamilyCreate(families))
	mux.HandleFunc("PUT /api/families/{id}", handleFamilyUpdate(families))
	mux.HandleFunc("DELETE /api/families/{id}", handleFamilyDelete(families))
}

// registerPersonRoutes регистрирует маршруты /api/people на переданном mux.
func registerPersonRoutes(mux *http.ServeMux, people PersonService) {
	mux.HandleFunc("GET /api/people", handlePersonList(people))
	mux.HandleFunc("GET /api/people/search", handlePersonSearch(people))
	mux.HandleFunc("GET /api/people/{id}", handlePersonGet(people))
	mux.HandleFunc("POST /api/people", handlePersonCreate(people))
	mux.HandleFunc("PUT /api/people/{id}", handlePersonUpdate(people))
	mux.HandleFunc("DELETE /api/people/{id}", handlePersonDelete(people))
}

// registerRelationRoutes регистрирует маршруты /api/relations на переданном
// mux. Без /search — search_relations намеренно не заводится (индекс пуст,
// см. docs/data-model/entity-write.md §3.8); список принимает необязательный
// ?person_id=.
func registerRelationRoutes(mux *http.ServeMux, relations RelationService) {
	mux.HandleFunc("GET /api/relations", handleRelationList(relations))
	mux.HandleFunc("GET /api/relations/{id}", handleRelationGet(relations))
	mux.HandleFunc("POST /api/relations", handleRelationCreate(relations))
	mux.HandleFunc("PUT /api/relations/{id}", handleRelationUpdate(relations))
	mux.HandleFunc("DELETE /api/relations/{id}", handleRelationDelete(relations))
}

// registerResidenceRoutes регистрирует маршруты /api/residences на
// переданном mux. Без /search (см. registerRelationRoutes); список
// принимает необязательные ?person_id=/?place_id=.
func registerResidenceRoutes(mux *http.ServeMux, residences ResidenceService) {
	mux.HandleFunc("GET /api/residences", handleResidenceList(residences))
	mux.HandleFunc("GET /api/residences/{id}", handleResidenceGet(residences))
	mux.HandleFunc("POST /api/residences", handleResidenceCreate(residences))
	mux.HandleFunc("PUT /api/residences/{id}", handleResidenceUpdate(residences))
	mux.HandleFunc("DELETE /api/residences/{id}", handleResidenceDelete(residences))
}

// registerEventRoutes регистрирует маршруты /api/events на переданном mux.
// Event ЕСТЬ поисковый индекс (по началу текста места) — полный набор из 6
// маршрутов, как у большинства сущностей; список принимает необязательный
// ?person_id=.
func registerEventRoutes(mux *http.ServeMux, events EventService) {
	mux.HandleFunc("GET /api/events", handleEventList(events))
	mux.HandleFunc("GET /api/events/search", handleEventSearch(events))
	mux.HandleFunc("GET /api/events/{id}", handleEventGet(events))
	mux.HandleFunc("POST /api/events", handleEventCreate(events))
	mux.HandleFunc("PUT /api/events/{id}", handleEventUpdate(events))
	mux.HandleFunc("DELETE /api/events/{id}", handleEventDelete(events))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// pathID читает {id} из пути запроса — общий хелпер для всех сущностей
// (division.go's pathDivisionID — исторический синоним, оставлен как есть).
func pathID(r *http.Request) models.ID {
	return models.ID(strings.TrimPrefix(r.PathValue("id"), "/"))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError отвечает на ошибку сценария: *models.ValidationError — 422 с полем,
// models.ErrNotFound — 404, *models.InUseError — 409 с телом InUseErrorBody,
// остальное — 500.
func writeError(w http.ResponseWriter, err error) {
	var ve *models.ValidationError
	if errors.As(err, &ve) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": ve.Error(), "field": ve.Field})

		return
	}

	if errors.Is(err, models.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})

		return
	}

	var iu *models.InUseError
	if errors.As(err, &iu) {
		writeJSON(w, http.StatusConflict, transport.InUseErrorBodyFromModel(iu))

		return
	}

	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}
