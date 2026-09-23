package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex"
	"github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/httpapi"
	"github.com/amarin/genodex/internal/idgen"
	"github.com/amarin/genodex/internal/mcp"
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store/sqlstore"
	create_archive "github.com/amarin/genodex/internal/usecases/create_archive"
	create_church "github.com/amarin/genodex/internal/usecases/create_church"
	create_division "github.com/amarin/genodex/internal/usecases/create_division"
	create_estate "github.com/amarin/genodex/internal/usecases/create_estate"
	create_given_name "github.com/amarin/genodex/internal/usecases/create_given_name"
	create_parish "github.com/amarin/genodex/internal/usecases/create_parish"
	create_patronymic "github.com/amarin/genodex/internal/usecases/create_patronymic"
	create_repository "github.com/amarin/genodex/internal/usecases/create_repository"
	create_surname "github.com/amarin/genodex/internal/usecases/create_surname"
	create_title "github.com/amarin/genodex/internal/usecases/create_title"
	delete_archive "github.com/amarin/genodex/internal/usecases/delete_archive"
	delete_church "github.com/amarin/genodex/internal/usecases/delete_church"
	delete_division "github.com/amarin/genodex/internal/usecases/delete_division"
	delete_estate "github.com/amarin/genodex/internal/usecases/delete_estate"
	delete_given_name "github.com/amarin/genodex/internal/usecases/delete_given_name"
	delete_parish "github.com/amarin/genodex/internal/usecases/delete_parish"
	delete_patronymic "github.com/amarin/genodex/internal/usecases/delete_patronymic"
	delete_repository "github.com/amarin/genodex/internal/usecases/delete_repository"
	delete_surname "github.com/amarin/genodex/internal/usecases/delete_surname"
	delete_title "github.com/amarin/genodex/internal/usecases/delete_title"
	get_archive "github.com/amarin/genodex/internal/usecases/get_archive"
	get_church "github.com/amarin/genodex/internal/usecases/get_church"
	get_division "github.com/amarin/genodex/internal/usecases/get_division"
	get_estate "github.com/amarin/genodex/internal/usecases/get_estate"
	get_given_name "github.com/amarin/genodex/internal/usecases/get_given_name"
	get_parish "github.com/amarin/genodex/internal/usecases/get_parish"
	get_patronymic "github.com/amarin/genodex/internal/usecases/get_patronymic"
	get_repository "github.com/amarin/genodex/internal/usecases/get_repository"
	get_surname "github.com/amarin/genodex/internal/usecases/get_surname"
	get_title "github.com/amarin/genodex/internal/usecases/get_title"
	list_archives "github.com/amarin/genodex/internal/usecases/list_archives"
	list_churches "github.com/amarin/genodex/internal/usecases/list_churches"
	list_divisions "github.com/amarin/genodex/internal/usecases/list_divisions"
	list_estates "github.com/amarin/genodex/internal/usecases/list_estates"
	list_given_names "github.com/amarin/genodex/internal/usecases/list_given_names"
	list_parishes "github.com/amarin/genodex/internal/usecases/list_parishes"
	list_patronymics "github.com/amarin/genodex/internal/usecases/list_patronymics"
	list_repositories "github.com/amarin/genodex/internal/usecases/list_repositories"
	list_surnames "github.com/amarin/genodex/internal/usecases/list_surnames"
	list_titles "github.com/amarin/genodex/internal/usecases/list_titles"
	search_archives "github.com/amarin/genodex/internal/usecases/search_archives"
	search_churches "github.com/amarin/genodex/internal/usecases/search_churches"
	search_divisions "github.com/amarin/genodex/internal/usecases/search_divisions"
	search_estates "github.com/amarin/genodex/internal/usecases/search_estates"
	search_given_names "github.com/amarin/genodex/internal/usecases/search_given_names"
	search_parishes "github.com/amarin/genodex/internal/usecases/search_parishes"
	search_patronymics "github.com/amarin/genodex/internal/usecases/search_patronymics"
	search_repositories "github.com/amarin/genodex/internal/usecases/search_repositories"
	search_surnames "github.com/amarin/genodex/internal/usecases/search_surnames"
	search_titles "github.com/amarin/genodex/internal/usecases/search_titles"
	update_archive "github.com/amarin/genodex/internal/usecases/update_archive"
	update_church "github.com/amarin/genodex/internal/usecases/update_church"
	update_division "github.com/amarin/genodex/internal/usecases/update_division"
	update_estate "github.com/amarin/genodex/internal/usecases/update_estate"
	update_given_name "github.com/amarin/genodex/internal/usecases/update_given_name"
	update_parish "github.com/amarin/genodex/internal/usecases/update_parish"
	update_patronymic "github.com/amarin/genodex/internal/usecases/update_patronymic"
	update_repository "github.com/amarin/genodex/internal/usecases/update_repository"
	update_surname "github.com/amarin/genodex/internal/usecases/update_surname"
	update_title "github.com/amarin/genodex/internal/usecases/update_title"
	"github.com/amarin/genodex/web"
)

// Config — параметры запуска приложения.
type Config struct {
	DataDir    string
	Port       int
	WebMode    string
	TrustProxy bool
}

// App — корневой объект приложения: собирает хранилище, usecases и интерфейсы.
type App struct {
	cfg   Config
	http  *http.Server
	store *sqlstore.Store
}

// divisionService — фасад всех сценариев делений, отдаваемых HTTP и MCP.
type divisionService struct {
	list   *list_divisions.Scenario
	search *search_divisions.Scenario
	get    *get_division.Scenario
	create *create_division.Scenario
	update *update_division.Scenario
	del    *delete_division.Scenario
}

func (s *divisionService) ListDivisions(ctx context.Context, access models.Access, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
	return s.list.ListDivisions(ctx, access, q)
}

func (s *divisionService) SearchDivisions(ctx context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error) {
	return s.search.SearchDivisions(ctx, access, q)
}

func (s *divisionService) GetDivision(ctx context.Context, id models.ID) (models.AdministrativeDivision, error) {
	return s.get.GetDivision(ctx, id)
}

func (s *divisionService) CreateDivision(ctx context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error) {
	return s.create.CreateDivision(ctx, d)
}

func (s *divisionService) UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error {
	return s.update.UpdateDivision(ctx, d)
}

func (s *divisionService) DeleteDivision(ctx context.Context, id models.ID) error {
	return s.del.DeleteDivision(ctx, id)
}

// surnameService — фасад всех сценариев словарных записей фамилий, отдаваемых
// HTTP и MCP. Тот же приём, что divisionService — по одному полю на
// сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type surnameService struct {
	list   *list_surnames.Scenario
	search *search_surnames.Scenario
	get    *get_surname.Scenario
	create *create_surname.Scenario
	update *update_surname.Scenario
	del    *delete_surname.Scenario
}

func (s *surnameService) ListSurnames(ctx context.Context, access models.Access, page models.Page) ([]models.Surname, error) {
	return s.list.ListSurnames(ctx, access, page)
}

func (s *surnameService) SearchSurnames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Surname, error) {
	return s.search.SearchSurnames(ctx, access, q)
}

func (s *surnameService) GetSurname(ctx context.Context, id models.ID) (models.Surname, error) {
	return s.get.GetSurname(ctx, id)
}

func (s *surnameService) CreateSurname(ctx context.Context, sn models.Surname) (models.Surname, error) {
	return s.create.CreateSurname(ctx, sn)
}

func (s *surnameService) UpdateSurname(ctx context.Context, sn models.Surname) error {
	return s.update.UpdateSurname(ctx, sn)
}

func (s *surnameService) DeleteSurname(ctx context.Context, id models.ID) error {
	return s.del.DeleteSurname(ctx, id)
}

// patronymicService — фасад всех сценариев словарных записей (отчеств), отдаваемых
// HTTP и MCP. Тот же приём, что divisionService/surnameService — по одному
// полю на сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type patronymicService struct {
	list   *list_patronymics.Scenario
	search *search_patronymics.Scenario
	get    *get_patronymic.Scenario
	create *create_patronymic.Scenario
	update *update_patronymic.Scenario
	del    *delete_patronymic.Scenario
}

func (s *patronymicService) ListPatronymics(ctx context.Context, access models.Access, page models.Page) ([]models.Patronymic, error) {
	return s.list.ListPatronymics(ctx, access, page)
}

func (s *patronymicService) SearchPatronymics(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Patronymic, error) {
	return s.search.SearchPatronymics(ctx, access, q)
}

func (s *patronymicService) GetPatronymic(ctx context.Context, id models.ID) (models.Patronymic, error) {
	return s.get.GetPatronymic(ctx, id)
}

func (s *patronymicService) CreatePatronymic(ctx context.Context, x models.Patronymic) (models.Patronymic, error) {
	return s.create.CreatePatronymic(ctx, x)
}

func (s *patronymicService) UpdatePatronymic(ctx context.Context, x models.Patronymic) error {
	return s.update.UpdatePatronymic(ctx, x)
}

func (s *patronymicService) DeletePatronymic(ctx context.Context, id models.ID) error {
	return s.del.DeletePatronymic(ctx, id)
}

// estateService — фасад всех сценариев словарных записей (сословий), отдаваемых
// HTTP и MCP. Тот же приём, что divisionService/surnameService — по одному
// полю на сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type estateService struct {
	list   *list_estates.Scenario
	search *search_estates.Scenario
	get    *get_estate.Scenario
	create *create_estate.Scenario
	update *update_estate.Scenario
	del    *delete_estate.Scenario
}

func (s *estateService) ListEstates(ctx context.Context, access models.Access, page models.Page) ([]models.Estate, error) {
	return s.list.ListEstates(ctx, access, page)
}

func (s *estateService) SearchEstates(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Estate, error) {
	return s.search.SearchEstates(ctx, access, q)
}

func (s *estateService) GetEstate(ctx context.Context, id models.ID) (models.Estate, error) {
	return s.get.GetEstate(ctx, id)
}

func (s *estateService) CreateEstate(ctx context.Context, x models.Estate) (models.Estate, error) {
	return s.create.CreateEstate(ctx, x)
}

func (s *estateService) UpdateEstate(ctx context.Context, x models.Estate) error {
	return s.update.UpdateEstate(ctx, x)
}

func (s *estateService) DeleteEstate(ctx context.Context, id models.ID) error {
	return s.del.DeleteEstate(ctx, id)
}

// titleService — фасад всех сценариев словарных записей (званий/титулов), отдаваемых
// HTTP и MCP. Тот же приём, что divisionService/surnameService — по одному
// полю на сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type titleService struct {
	list   *list_titles.Scenario
	search *search_titles.Scenario
	get    *get_title.Scenario
	create *create_title.Scenario
	update *update_title.Scenario
	del    *delete_title.Scenario
}

func (s *titleService) ListTitles(ctx context.Context, access models.Access, page models.Page) ([]models.Title, error) {
	return s.list.ListTitles(ctx, access, page)
}

func (s *titleService) SearchTitles(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Title, error) {
	return s.search.SearchTitles(ctx, access, q)
}

func (s *titleService) GetTitle(ctx context.Context, id models.ID) (models.Title, error) {
	return s.get.GetTitle(ctx, id)
}

func (s *titleService) CreateTitle(ctx context.Context, x models.Title) (models.Title, error) {
	return s.create.CreateTitle(ctx, x)
}

func (s *titleService) UpdateTitle(ctx context.Context, x models.Title) error {
	return s.update.UpdateTitle(ctx, x)
}

func (s *titleService) DeleteTitle(ctx context.Context, id models.ID) error {
	return s.del.DeleteTitle(ctx, id)
}

// givenNameService — фасад всех сценариев словарных записей (имён), отдаваемых
// HTTP и MCP. Тот же приём, что divisionService/surnameService — по одному
// полю на сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type givenNameService struct {
	list   *list_given_names.Scenario
	search *search_given_names.Scenario
	get    *get_given_name.Scenario
	create *create_given_name.Scenario
	update *update_given_name.Scenario
	del    *delete_given_name.Scenario
}

func (s *givenNameService) ListGivenNames(ctx context.Context, access models.Access, page models.Page) ([]models.GivenName, error) {
	return s.list.ListGivenNames(ctx, access, page)
}

func (s *givenNameService) SearchGivenNames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.GivenName, error) {
	return s.search.SearchGivenNames(ctx, access, q)
}

func (s *givenNameService) GetGivenName(ctx context.Context, id models.ID) (models.GivenName, error) {
	return s.get.GetGivenName(ctx, id)
}

func (s *givenNameService) CreateGivenName(ctx context.Context, x models.GivenName) (models.GivenName, error) {
	return s.create.CreateGivenName(ctx, x)
}

func (s *givenNameService) UpdateGivenName(ctx context.Context, x models.GivenName) error {
	return s.update.UpdateGivenName(ctx, x)
}

func (s *givenNameService) DeleteGivenName(ctx context.Context, id models.ID) error {
	return s.del.DeleteGivenName(ctx, id)
}

// repositoryService — фасад всех сценариев хранилищ-контейнеров источников,
// отдаваемых HTTP и MCP. Тот же приём, что divisionService/surnameService —
// по одному полю на сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type repositoryService struct {
	list   *list_repositories.Scenario
	search *search_repositories.Scenario
	get    *get_repository.Scenario
	create *create_repository.Scenario
	update *update_repository.Scenario
	del    *delete_repository.Scenario
}

func (s *repositoryService) ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]models.Repository, error) {
	return s.list.ListRepositories(ctx, access, page)
}

func (s *repositoryService) SearchRepositories(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Repository, error) {
	return s.search.SearchRepositories(ctx, access, q)
}

func (s *repositoryService) GetRepository(ctx context.Context, id models.ID) (models.Repository, error) {
	return s.get.GetRepository(ctx, id)
}

func (s *repositoryService) CreateRepository(ctx context.Context, r models.Repository) (models.Repository, error) {
	return s.create.CreateRepository(ctx, r)
}

func (s *repositoryService) UpdateRepository(ctx context.Context, r models.Repository) error {
	return s.update.UpdateRepository(ctx, r)
}

func (s *repositoryService) DeleteRepository(ctx context.Context, id models.ID) error {
	return s.del.DeleteRepository(ctx, id)
}

// churchService — фасад всех сценариев церквей, отдаваемых HTTP и MCP.
type churchService struct {
	list   *list_churches.Scenario
	search *search_churches.Scenario
	get    *get_church.Scenario
	create *create_church.Scenario
	update *update_church.Scenario
	del    *delete_church.Scenario
}

func (s *churchService) ListChurches(ctx context.Context, access models.Access, page models.Page) ([]models.Church, error) {
	return s.list.ListChurches(ctx, access, page)
}

func (s *churchService) SearchChurches(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Church, error) {
	return s.search.SearchChurches(ctx, access, q)
}

func (s *churchService) GetChurch(ctx context.Context, id models.ID) (models.Church, error) {
	return s.get.GetChurch(ctx, id)
}

func (s *churchService) CreateChurch(ctx context.Context, c models.Church) (models.Church, error) {
	return s.create.CreateChurch(ctx, c)
}

func (s *churchService) UpdateChurch(ctx context.Context, c models.Church) error {
	return s.update.UpdateChurch(ctx, c)
}

func (s *churchService) DeleteChurch(ctx context.Context, id models.ID) error {
	return s.del.DeleteChurch(ctx, id)
}

// parishService — фасад всех сценариев приходов, отдаваемых HTTP и MCP.
type parishService struct {
	list   *list_parishes.Scenario
	search *search_parishes.Scenario
	get    *get_parish.Scenario
	create *create_parish.Scenario
	update *update_parish.Scenario
	del    *delete_parish.Scenario
}

func (s *parishService) ListParishes(ctx context.Context, access models.Access, page models.Page) ([]models.Parish, error) {
	return s.list.ListParishes(ctx, access, page)
}

func (s *parishService) SearchParishes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Parish, error) {
	return s.search.SearchParishes(ctx, access, q)
}

func (s *parishService) GetParish(ctx context.Context, id models.ID) (models.Parish, error) {
	return s.get.GetParish(ctx, id)
}

func (s *parishService) CreateParish(ctx context.Context, p models.Parish) (models.Parish, error) {
	return s.create.CreateParish(ctx, p)
}

func (s *parishService) UpdateParish(ctx context.Context, p models.Parish) error {
	return s.update.UpdateParish(ctx, p)
}

func (s *parishService) DeleteParish(ctx context.Context, id models.ID) error {
	return s.del.DeleteParish(ctx, id)
}

// archiveService — фасад всех сценариев архивов, отдаваемых HTTP и MCP.
type archiveService struct {
	list   *list_archives.Scenario
	search *search_archives.Scenario
	get    *get_archive.Scenario
	create *create_archive.Scenario
	update *update_archive.Scenario
	del    *delete_archive.Scenario
}

func (s *archiveService) ListArchives(ctx context.Context, access models.Access, page models.Page) ([]models.Archive, error) {
	return s.list.ListArchives(ctx, access, page)
}

func (s *archiveService) SearchArchives(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Archive, error) {
	return s.search.SearchArchives(ctx, access, q)
}

func (s *archiveService) GetArchive(ctx context.Context, id models.ID) (models.Archive, error) {
	return s.get.GetArchive(ctx, id)
}

func (s *archiveService) CreateArchive(ctx context.Context, a models.Archive) (models.Archive, error) {
	return s.create.CreateArchive(ctx, a)
}

func (s *archiveService) UpdateArchive(ctx context.Context, a models.Archive) error {
	return s.update.UpdateArchive(ctx, a)
}

func (s *archiveService) DeleteArchive(ctx context.Context, id models.ID) error {
	return s.del.DeleteArchive(ctx, id)
}

var (
	_ httpapi.DivisionService   = (*divisionService)(nil)
	_ mcp.DivisionService       = (*divisionService)(nil)
	_ httpapi.SurnameService    = (*surnameService)(nil)
	_ mcp.SurnameService        = (*surnameService)(nil)
	_ httpapi.PatronymicService = (*patronymicService)(nil)
	_ mcp.PatronymicService     = (*patronymicService)(nil)
	_ httpapi.EstateService     = (*estateService)(nil)
	_ mcp.EstateService         = (*estateService)(nil)
	_ httpapi.TitleService      = (*titleService)(nil)
	_ mcp.TitleService          = (*titleService)(nil)
	_ httpapi.GivenNameService  = (*givenNameService)(nil)
	_ mcp.GivenNameService      = (*givenNameService)(nil)
	_ httpapi.RepositoryService = (*repositoryService)(nil)
	_ mcp.RepositoryService     = (*repositoryService)(nil)
	_ httpapi.ChurchService     = (*churchService)(nil)
	_ mcp.ChurchService         = (*churchService)(nil)
	_ httpapi.ParishService     = (*parishService)(nil)
	_ mcp.ParishService         = (*parishService)(nil)
	_ httpapi.ArchiveService    = (*archiveService)(nil)
	_ mcp.ArchiveService        = (*archiveService)(nil)
	_ httpapi.AuthService       = (*auth.Service)(nil)
	_ mcp.TokenResolver         = (*auth.Service)(nil)
)

// New собирает приложение: хранилище → сценарии/auth → MCP/HTTP интерфейсы.
func New(cfg Config) (*App, error) {
	st, err := sqlstore.Open(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	divisions := &divisionService{
		list:   list_divisions.New(st),
		search: search_divisions.New(st),
		get:    get_division.New(st),
		create: create_division.New(st, idgen.New()),
		update: update_division.New(st),
		del:    delete_division.New(st),
	}

	surnames := &surnameService{
		list:   list_surnames.New(st),
		search: search_surnames.New(st),
		get:    get_surname.New(st),
		create: create_surname.New(st, idgen.New()),
		update: update_surname.New(st),
		del:    delete_surname.New(st),
	}

	patronymics := &patronymicService{
		list:   list_patronymics.New(st),
		search: search_patronymics.New(st),
		get:    get_patronymic.New(st),
		create: create_patronymic.New(st, idgen.New()),
		update: update_patronymic.New(st),
		del:    delete_patronymic.New(st),
	}

	estates := &estateService{
		list:   list_estates.New(st),
		search: search_estates.New(st),
		get:    get_estate.New(st),
		create: create_estate.New(st, idgen.New()),
		update: update_estate.New(st),
		del:    delete_estate.New(st),
	}

	titles := &titleService{
		list:   list_titles.New(st),
		search: search_titles.New(st),
		get:    get_title.New(st),
		create: create_title.New(st, idgen.New()),
		update: update_title.New(st),
		del:    delete_title.New(st),
	}

	givenNames := &givenNameService{
		list:   list_given_names.New(st),
		search: search_given_names.New(st),
		get:    get_given_name.New(st),
		create: create_given_name.New(st, idgen.New()),
		update: update_given_name.New(st),
		del:    delete_given_name.New(st),
	}

	repositories := &repositoryService{
		list:   list_repositories.New(st),
		search: search_repositories.New(st),
		get:    get_repository.New(st),
		create: create_repository.New(st, idgen.New()),
		update: update_repository.New(st),
		del:    delete_repository.New(st),
	}

	churches := &churchService{
		list:   list_churches.New(st),
		search: search_churches.New(st),
		get:    get_church.New(st),
		create: create_church.New(st, idgen.New()),
		update: update_church.New(st),
		del:    delete_church.New(st),
	}

	parishes := &parishService{
		list:   list_parishes.New(st),
		search: search_parishes.New(st),
		get:    get_parish.New(st),
		create: create_parish.New(st, idgen.New()),
		update: update_parish.New(st),
		del:    delete_parish.New(st),
	}

	archives := &archiveService{
		list:   list_archives.New(st),
		search: search_archives.New(st),
		get:    get_archive.New(st),
		create: create_archive.New(st, idgen.New()),
		update: update_archive.New(st),
		del:    delete_archive.New(st),
	}

	// auth-хранилище — на том же соединении, что и общий store (см.
	// sqlstore.Store.DB), файл БД один и тот же (internal/storage/schema_auth.go).
	authService := auth.New(auth.NewSQLStore(st.DB()))

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.RequireAPIToken(authService)(server.NewStreamableHTTPServer(
		mcp.NewServer(mcp.Deps{
			Divisions: divisions, Surnames: surnames,
			Patronymics: patronymics, Estates: estates, Titles: titles, GivenNames: givenNames,
			Repositories: repositories, Churches: churches, Parishes: parishes, Archives: archives,
		}),
	)))
	mux.Handle("/api/", httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    divisions,
		Surnames:     surnames,
		Patronymics:  patronymics,
		Estates:      estates,
		Titles:       titles,
		GivenNames:   givenNames,
		Repositories: repositories,
		Churches:     churches,
		Parishes:     parishes,
		Archives:     archives,
		Auth:         authService,
		DocsFS:       genodex.DocsFS(cfg.WebMode),
		TrustProxy:   cfg.TrustProxy,
	}))
	mux.Handle("/static/", http.StripPrefix("/static/", web.StaticHandler(cfg.WebMode)))
	mux.Handle("/", web.SPAHandler(cfg.WebMode))

	return &App{
		cfg: cfg,
		http: &http.Server{
			Addr:    fmt.Sprintf("0.0.0.0:%d", cfg.Port),
			Handler: mux,
		},
		store: st,
	}, nil
}

// Run запускает HTTP-сервер. Останавливается по отмене ctx (graceful shutdown
// HTTP и закрытие хранилища) либо по ошибке ListenAndServe.
func (a *App) Run(ctx context.Context) error {
	log.Printf("Genealogy MCP server started on %s", a.http.Addr)
	log.Printf("  MCP:      http://localhost:%d/mcp", a.cfg.Port)
	log.Printf("  API:      http://localhost:%d/api", a.cfg.Port)
	log.Printf("  Web:      http://localhost:%d/ (web mode: %s)", a.cfg.Port, a.cfg.WebMode)
	log.Printf("  Trust proxy (X-Forwarded-Proto): %v", a.cfg.TrustProxy)

	go func() {
		<-ctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.http.Shutdown(shCtx); err != nil {
			log.Printf("Graceful shutdown error: %v", err)
		}
		if err := a.store.Close(); err != nil {
			log.Printf("Store close error: %v", err)
		}
	}()

	if err := a.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
