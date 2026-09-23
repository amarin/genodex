package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/httpapi"
	"github.com/amarin/genodex/internal/idgen"
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store/sqlstore"
	create_archive "github.com/amarin/genodex/internal/usecases/create_archive"
	create_archive_document "github.com/amarin/genodex/internal/usecases/create_archive_document"
	create_archive_node "github.com/amarin/genodex/internal/usecases/create_archive_node"
	create_attachment "github.com/amarin/genodex/internal/usecases/create_attachment"
	create_church "github.com/amarin/genodex/internal/usecases/create_church"
	create_citation "github.com/amarin/genodex/internal/usecases/create_citation"
	create_division "github.com/amarin/genodex/internal/usecases/create_division"
	create_estate "github.com/amarin/genodex/internal/usecases/create_estate"
	create_family "github.com/amarin/genodex/internal/usecases/create_family"
	create_given_name "github.com/amarin/genodex/internal/usecases/create_given_name"
	create_note "github.com/amarin/genodex/internal/usecases/create_note"
	create_parish "github.com/amarin/genodex/internal/usecases/create_parish"
	create_patronymic "github.com/amarin/genodex/internal/usecases/create_patronymic"
	create_repository "github.com/amarin/genodex/internal/usecases/create_repository"
	create_source "github.com/amarin/genodex/internal/usecases/create_source"
	create_surname "github.com/amarin/genodex/internal/usecases/create_surname"
	create_title "github.com/amarin/genodex/internal/usecases/create_title"
	delete_archive "github.com/amarin/genodex/internal/usecases/delete_archive"
	delete_archive_document "github.com/amarin/genodex/internal/usecases/delete_archive_document"
	delete_archive_node "github.com/amarin/genodex/internal/usecases/delete_archive_node"
	delete_attachment "github.com/amarin/genodex/internal/usecases/delete_attachment"
	delete_church "github.com/amarin/genodex/internal/usecases/delete_church"
	delete_citation "github.com/amarin/genodex/internal/usecases/delete_citation"
	delete_division "github.com/amarin/genodex/internal/usecases/delete_division"
	delete_estate "github.com/amarin/genodex/internal/usecases/delete_estate"
	delete_family "github.com/amarin/genodex/internal/usecases/delete_family"
	delete_given_name "github.com/amarin/genodex/internal/usecases/delete_given_name"
	delete_note "github.com/amarin/genodex/internal/usecases/delete_note"
	delete_parish "github.com/amarin/genodex/internal/usecases/delete_parish"
	delete_patronymic "github.com/amarin/genodex/internal/usecases/delete_patronymic"
	delete_repository "github.com/amarin/genodex/internal/usecases/delete_repository"
	delete_source "github.com/amarin/genodex/internal/usecases/delete_source"
	delete_surname "github.com/amarin/genodex/internal/usecases/delete_surname"
	delete_title "github.com/amarin/genodex/internal/usecases/delete_title"
	get_archive "github.com/amarin/genodex/internal/usecases/get_archive"
	get_archive_document "github.com/amarin/genodex/internal/usecases/get_archive_document"
	get_archive_node "github.com/amarin/genodex/internal/usecases/get_archive_node"
	get_attachment "github.com/amarin/genodex/internal/usecases/get_attachment"
	get_church "github.com/amarin/genodex/internal/usecases/get_church"
	get_citation "github.com/amarin/genodex/internal/usecases/get_citation"
	get_division "github.com/amarin/genodex/internal/usecases/get_division"
	get_estate "github.com/amarin/genodex/internal/usecases/get_estate"
	get_family "github.com/amarin/genodex/internal/usecases/get_family"
	get_given_name "github.com/amarin/genodex/internal/usecases/get_given_name"
	get_note "github.com/amarin/genodex/internal/usecases/get_note"
	get_parish "github.com/amarin/genodex/internal/usecases/get_parish"
	get_patronymic "github.com/amarin/genodex/internal/usecases/get_patronymic"
	get_repository "github.com/amarin/genodex/internal/usecases/get_repository"
	get_source "github.com/amarin/genodex/internal/usecases/get_source"
	get_surname "github.com/amarin/genodex/internal/usecases/get_surname"
	get_title "github.com/amarin/genodex/internal/usecases/get_title"
	list_archive_documents "github.com/amarin/genodex/internal/usecases/list_archive_documents"
	list_archive_nodes "github.com/amarin/genodex/internal/usecases/list_archive_nodes"
	list_archives "github.com/amarin/genodex/internal/usecases/list_archives"
	list_attachments "github.com/amarin/genodex/internal/usecases/list_attachments"
	list_churches "github.com/amarin/genodex/internal/usecases/list_churches"
	list_citations "github.com/amarin/genodex/internal/usecases/list_citations"
	list_divisions "github.com/amarin/genodex/internal/usecases/list_divisions"
	list_estates "github.com/amarin/genodex/internal/usecases/list_estates"
	list_families "github.com/amarin/genodex/internal/usecases/list_families"
	list_given_names "github.com/amarin/genodex/internal/usecases/list_given_names"
	list_notes "github.com/amarin/genodex/internal/usecases/list_notes"
	list_parishes "github.com/amarin/genodex/internal/usecases/list_parishes"
	list_patronymics "github.com/amarin/genodex/internal/usecases/list_patronymics"
	list_repositories "github.com/amarin/genodex/internal/usecases/list_repositories"
	list_sources "github.com/amarin/genodex/internal/usecases/list_sources"
	list_surnames "github.com/amarin/genodex/internal/usecases/list_surnames"
	list_titles "github.com/amarin/genodex/internal/usecases/list_titles"
	search_archive_documents "github.com/amarin/genodex/internal/usecases/search_archive_documents"
	search_archive_nodes "github.com/amarin/genodex/internal/usecases/search_archive_nodes"
	search_archives "github.com/amarin/genodex/internal/usecases/search_archives"
	search_attachments "github.com/amarin/genodex/internal/usecases/search_attachments"
	search_churches "github.com/amarin/genodex/internal/usecases/search_churches"
	search_citations "github.com/amarin/genodex/internal/usecases/search_citations"
	search_divisions "github.com/amarin/genodex/internal/usecases/search_divisions"
	search_estates "github.com/amarin/genodex/internal/usecases/search_estates"
	search_families "github.com/amarin/genodex/internal/usecases/search_families"
	search_given_names "github.com/amarin/genodex/internal/usecases/search_given_names"
	search_notes "github.com/amarin/genodex/internal/usecases/search_notes"
	search_parishes "github.com/amarin/genodex/internal/usecases/search_parishes"
	search_patronymics "github.com/amarin/genodex/internal/usecases/search_patronymics"
	search_repositories "github.com/amarin/genodex/internal/usecases/search_repositories"
	search_sources "github.com/amarin/genodex/internal/usecases/search_sources"
	search_surnames "github.com/amarin/genodex/internal/usecases/search_surnames"
	search_titles "github.com/amarin/genodex/internal/usecases/search_titles"
	update_archive "github.com/amarin/genodex/internal/usecases/update_archive"
	update_archive_document "github.com/amarin/genodex/internal/usecases/update_archive_document"
	update_archive_node "github.com/amarin/genodex/internal/usecases/update_archive_node"
	update_attachment "github.com/amarin/genodex/internal/usecases/update_attachment"
	update_church "github.com/amarin/genodex/internal/usecases/update_church"
	update_citation "github.com/amarin/genodex/internal/usecases/update_citation"
	update_division "github.com/amarin/genodex/internal/usecases/update_division"
	update_estate "github.com/amarin/genodex/internal/usecases/update_estate"
	update_family "github.com/amarin/genodex/internal/usecases/update_family"
	update_given_name "github.com/amarin/genodex/internal/usecases/update_given_name"
	update_note "github.com/amarin/genodex/internal/usecases/update_note"
	update_parish "github.com/amarin/genodex/internal/usecases/update_parish"
	update_patronymic "github.com/amarin/genodex/internal/usecases/update_patronymic"
	update_repository "github.com/amarin/genodex/internal/usecases/update_repository"
	update_source "github.com/amarin/genodex/internal/usecases/update_source"
	update_surname "github.com/amarin/genodex/internal/usecases/update_surname"
	update_title "github.com/amarin/genodex/internal/usecases/update_title"
)

// divisionService — сборка httpapi.DivisionService на настоящих сценариях
// (так же собран internal/app).
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

// newDivisionService собирает фасад на настоящем хранилище.
func newDivisionService(t *testing.T, st *sqlstore.Store) *divisionService {
	t.Helper()

	return &divisionService{
		list:   list_divisions.New(st),
		search: search_divisions.New(st),
		get:    get_division.New(st),
		create: create_division.New(st, idgen.New()),
		update: update_division.New(st),
		del:    delete_division.New(st),
	}
}

// surnameService — сборка httpapi.SurnameService на настоящих сценариях
// (так же собран internal/app's surnameService).
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

// newSurnameService собирает фасад на настоящем хранилище.
func newSurnameService(t *testing.T, st *sqlstore.Store) *surnameService {
	t.Helper()

	return &surnameService{
		list:   list_surnames.New(st),
		search: search_surnames.New(st),
		get:    get_surname.New(st),
		create: create_surname.New(st, idgen.New()),
		update: update_surname.New(st),
		del:    delete_surname.New(st),
	}
}

// patronymicService — сборка httpapi.PatronymicService на настоящих сценариях
// (так же собран internal/app's patronymicService).
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

// newPatronymicService собирает фасад на настоящем хранилище.
func newPatronymicService(t *testing.T, st *sqlstore.Store) *patronymicService {
	t.Helper()

	return &patronymicService{
		list:   list_patronymics.New(st),
		search: search_patronymics.New(st),
		get:    get_patronymic.New(st),
		create: create_patronymic.New(st, idgen.New()),
		update: update_patronymic.New(st),
		del:    delete_patronymic.New(st),
	}
}

// estateService — сборка httpapi.EstateService на настоящих сценариях
// (так же собран internal/app's estateService).
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

// newEstateService собирает фасад на настоящем хранилище.
func newEstateService(t *testing.T, st *sqlstore.Store) *estateService {
	t.Helper()

	return &estateService{
		list:   list_estates.New(st),
		search: search_estates.New(st),
		get:    get_estate.New(st),
		create: create_estate.New(st, idgen.New()),
		update: update_estate.New(st),
		del:    delete_estate.New(st),
	}
}

// titleService — сборка httpapi.TitleService на настоящих сценариях
// (так же собран internal/app's titleService).
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

// newTitleService собирает фасад на настоящем хранилище.
func newTitleService(t *testing.T, st *sqlstore.Store) *titleService {
	t.Helper()

	return &titleService{
		list:   list_titles.New(st),
		search: search_titles.New(st),
		get:    get_title.New(st),
		create: create_title.New(st, idgen.New()),
		update: update_title.New(st),
		del:    delete_title.New(st),
	}
}

// givenNameService — сборка httpapi.GivenNameService на настоящих сценариях
// (так же собран internal/app's givenNameService).
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

// newGivenNameService собирает фасад на настоящем хранилище.
func newGivenNameService(t *testing.T, st *sqlstore.Store) *givenNameService {
	t.Helper()

	return &givenNameService{
		list:   list_given_names.New(st),
		search: search_given_names.New(st),
		get:    get_given_name.New(st),
		create: create_given_name.New(st, idgen.New()),
		update: update_given_name.New(st),
		del:    delete_given_name.New(st),
	}
}

// familyService — фасад httpapi.FamilyService на настоящих сценариях (так же
// собран internal/app's familyService).
type familyService struct {
	list   *list_families.Scenario
	search *search_families.Scenario
	get    *get_family.Scenario
	create *create_family.Scenario
	update *update_family.Scenario
	del    *delete_family.Scenario
}

func (s *familyService) ListFamilies(ctx context.Context, access models.Access, page models.Page) ([]models.Family, error) {
	return s.list.ListFamilies(ctx, access, page)
}

func (s *familyService) SearchFamilies(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Family, error) {
	return s.search.SearchFamilies(ctx, access, q)
}

func (s *familyService) GetFamily(ctx context.Context, access models.Access, id models.ID) (models.Family, error) {
	return s.get.GetFamily(ctx, access, id)
}

func (s *familyService) CreateFamily(ctx context.Context, f models.Family) (models.Family, error) {
	return s.create.CreateFamily(ctx, f)
}

func (s *familyService) UpdateFamily(ctx context.Context, f models.Family) error {
	return s.update.UpdateFamily(ctx, f)
}

func (s *familyService) DeleteFamily(ctx context.Context, id models.ID) error {
	return s.del.DeleteFamily(ctx, id)
}

// newFamilyService собирает фасад на настоящем хранилище.
func newFamilyService(t *testing.T, st *sqlstore.Store) *familyService {
	t.Helper()

	return &familyService{
		list:   list_families.New(st),
		search: search_families.New(st),
		get:    get_family.New(st),
		create: create_family.New(st, idgen.New()),
		update: update_family.New(st),
		del:    delete_family.New(st),
	}
}

// repositoryService — фасад httpapi.RepositoryService на настоящих сценариях
// (так же собран internal/app's repositoryService).
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

func (s *repositoryService) GetRepository(ctx context.Context, access models.Access, id models.ID) (models.Repository, error) {
	return s.get.GetRepository(ctx, access, id)
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

// newRepositoryService собирает фасад на настоящем хранилище.
func newRepositoryService(t *testing.T, st *sqlstore.Store) *repositoryService {
	t.Helper()

	return &repositoryService{
		list:   list_repositories.New(st),
		search: search_repositories.New(st),
		get:    get_repository.New(st),
		create: create_repository.New(st, idgen.New()),
		update: update_repository.New(st),
		del:    delete_repository.New(st),
	}
}

// churchService — фасад httpapi.ChurchService на настоящих сценариях (так же
// собран internal/app's churchService).
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

// newChurchService собирает фасад на настоящем хранилище.
func newChurchService(t *testing.T, st *sqlstore.Store) *churchService {
	t.Helper()

	return &churchService{
		list:   list_churches.New(st),
		search: search_churches.New(st),
		get:    get_church.New(st),
		create: create_church.New(st, idgen.New()),
		update: update_church.New(st),
		del:    delete_church.New(st),
	}
}

// parishService — фасад httpapi.ParishService на настоящих сценариях (так же
// собран internal/app's parishService).
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

// newParishService собирает фасад на настоящем хранилище.
func newParishService(t *testing.T, st *sqlstore.Store) *parishService {
	t.Helper()

	return &parishService{
		list:   list_parishes.New(st),
		search: search_parishes.New(st),
		get:    get_parish.New(st),
		create: create_parish.New(st, idgen.New()),
		update: update_parish.New(st),
		del:    delete_parish.New(st),
	}
}

// archiveService — фасад httpapi.ArchiveService на настоящих сценариях (так
// же собран internal/app's archiveService).
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

func (s *archiveService) GetArchive(ctx context.Context, access models.Access, id models.ID) (models.Archive, error) {
	return s.get.GetArchive(ctx, access, id)
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

// newArchiveService собирает фасад на настоящем хранилище.
func newArchiveService(t *testing.T, st *sqlstore.Store) *archiveService {
	t.Helper()

	return &archiveService{
		list:   list_archives.New(st),
		search: search_archives.New(st),
		get:    get_archive.New(st),
		create: create_archive.New(st, idgen.New()),
		update: update_archive.New(st),
		del:    delete_archive.New(st),
	}
}

// archiveNodeService — фасад httpapi.ArchiveNodeService на настоящих
// сценариях (так же собран internal/app's archiveNodeService).
type archiveNodeService struct {
	list   *list_archive_nodes.Scenario
	search *search_archive_nodes.Scenario
	get    *get_archive_node.Scenario
	create *create_archive_node.Scenario
	update *update_archive_node.Scenario
	del    *delete_archive_node.Scenario
}

func (s *archiveNodeService) ListArchiveNodes(ctx context.Context, access models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error) {
	return s.list.ListArchiveNodes(ctx, access, q)
}

func (s *archiveNodeService) SearchArchiveNodes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveNode, error) {
	return s.search.SearchArchiveNodes(ctx, access, q)
}

func (s *archiveNodeService) GetArchiveNode(ctx context.Context, access models.Access, id models.ID) (models.ArchiveNode, error) {
	return s.get.GetArchiveNode(ctx, access, id)
}

func (s *archiveNodeService) CreateArchiveNode(ctx context.Context, n models.ArchiveNode) (models.ArchiveNode, error) {
	return s.create.CreateArchiveNode(ctx, n)
}

func (s *archiveNodeService) UpdateArchiveNode(ctx context.Context, n models.ArchiveNode) error {
	return s.update.UpdateArchiveNode(ctx, n)
}

func (s *archiveNodeService) DeleteArchiveNode(ctx context.Context, id models.ID) error {
	return s.del.DeleteArchiveNode(ctx, id)
}

// newArchiveNodeService собирает фасад на настоящем хранилище.
func newArchiveNodeService(t *testing.T, st *sqlstore.Store) *archiveNodeService {
	t.Helper()

	return &archiveNodeService{
		list:   list_archive_nodes.New(st),
		search: search_archive_nodes.New(st),
		get:    get_archive_node.New(st),
		create: create_archive_node.New(st, idgen.New()),
		update: update_archive_node.New(st),
		del:    delete_archive_node.New(st),
	}
}

// archiveDocumentService — фасад httpapi.ArchiveDocumentService на настоящих
// сценариях (так же собран internal/app's archiveDocumentService).
type archiveDocumentService struct {
	list   *list_archive_documents.Scenario
	search *search_archive_documents.Scenario
	get    *get_archive_document.Scenario
	create *create_archive_document.Scenario
	update *update_archive_document.Scenario
	del    *delete_archive_document.Scenario
}

func (s *archiveDocumentService) ListArchiveDocuments(ctx context.Context, access models.Access, page models.Page) ([]models.ArchiveDocument, error) {
	return s.list.ListArchiveDocuments(ctx, access, page)
}

func (s *archiveDocumentService) SearchArchiveDocuments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveDocument, error) {
	return s.search.SearchArchiveDocuments(ctx, access, q)
}

func (s *archiveDocumentService) GetArchiveDocument(ctx context.Context, access models.Access, id models.ID) (models.ArchiveDocument, error) {
	return s.get.GetArchiveDocument(ctx, access, id)
}

func (s *archiveDocumentService) CreateArchiveDocument(ctx context.Context, d models.ArchiveDocument) (models.ArchiveDocument, error) {
	return s.create.CreateArchiveDocument(ctx, d)
}

func (s *archiveDocumentService) UpdateArchiveDocument(ctx context.Context, d models.ArchiveDocument) error {
	return s.update.UpdateArchiveDocument(ctx, d)
}

func (s *archiveDocumentService) DeleteArchiveDocument(ctx context.Context, id models.ID) error {
	return s.del.DeleteArchiveDocument(ctx, id)
}

// newArchiveDocumentService собирает фасад на настоящем хранилище.
func newArchiveDocumentService(t *testing.T, st *sqlstore.Store) *archiveDocumentService {
	t.Helper()

	return &archiveDocumentService{
		list:   list_archive_documents.New(st),
		search: search_archive_documents.New(st),
		get:    get_archive_document.New(st),
		create: create_archive_document.New(st, idgen.New()),
		update: update_archive_document.New(st),
		del:    delete_archive_document.New(st),
	}
}

// sourceService — фасад httpapi.SourceService на настоящих сценариях (так же
// собран internal/app's sourceService).
type sourceService struct {
	list   *list_sources.Scenario
	search *search_sources.Scenario
	get    *get_source.Scenario
	create *create_source.Scenario
	update *update_source.Scenario
	del    *delete_source.Scenario
}

func (s *sourceService) ListSources(ctx context.Context, access models.Access, page models.Page) ([]models.Source, error) {
	return s.list.ListSources(ctx, access, page)
}

func (s *sourceService) SearchSources(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Source, error) {
	return s.search.SearchSources(ctx, access, q)
}

func (s *sourceService) GetSource(ctx context.Context, access models.Access, id models.ID) (models.Source, error) {
	return s.get.GetSource(ctx, access, id)
}

func (s *sourceService) CreateSource(ctx context.Context, src models.Source) (models.Source, error) {
	return s.create.CreateSource(ctx, src)
}

func (s *sourceService) UpdateSource(ctx context.Context, src models.Source) error {
	return s.update.UpdateSource(ctx, src)
}

func (s *sourceService) DeleteSource(ctx context.Context, id models.ID) error {
	return s.del.DeleteSource(ctx, id)
}

// newSourceService собирает фасад на настоящем хранилище.
func newSourceService(t *testing.T, st *sqlstore.Store) *sourceService {
	t.Helper()

	return &sourceService{
		list:   list_sources.New(st),
		search: search_sources.New(st),
		get:    get_source.New(st),
		create: create_source.New(st, idgen.New()),
		update: update_source.New(st),
		del:    delete_source.New(st),
	}
}

// citationService — фасад httpapi.CitationService на настоящих сценариях
// (так же собран internal/app's citationService).
type citationService struct {
	list   *list_citations.Scenario
	search *search_citations.Scenario
	get    *get_citation.Scenario
	create *create_citation.Scenario
	update *update_citation.Scenario
	del    *delete_citation.Scenario
}

func (s *citationService) ListCitations(ctx context.Context, access models.Access, page models.Page) ([]models.Citation, error) {
	return s.list.ListCitations(ctx, access, page)
}

func (s *citationService) SearchCitations(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Citation, error) {
	return s.search.SearchCitations(ctx, access, q)
}

func (s *citationService) GetCitation(ctx context.Context, access models.Access, id models.ID) (models.Citation, error) {
	return s.get.GetCitation(ctx, access, id)
}

func (s *citationService) CreateCitation(ctx context.Context, c models.Citation) (models.Citation, error) {
	return s.create.CreateCitation(ctx, c)
}

func (s *citationService) UpdateCitation(ctx context.Context, c models.Citation) error {
	return s.update.UpdateCitation(ctx, c)
}

func (s *citationService) DeleteCitation(ctx context.Context, id models.ID) error {
	return s.del.DeleteCitation(ctx, id)
}

// newCitationService собирает фасад на настоящем хранилище.
func newCitationService(t *testing.T, st *sqlstore.Store) *citationService {
	t.Helper()

	return &citationService{
		list:   list_citations.New(st),
		search: search_citations.New(st),
		get:    get_citation.New(st),
		create: create_citation.New(st, idgen.New()),
		update: update_citation.New(st),
		del:    delete_citation.New(st),
	}
}

// noteService — фасад httpapi.NoteService на настоящих сценариях (так же
// собран internal/app's noteService).
type noteService struct {
	list   *list_notes.Scenario
	search *search_notes.Scenario
	get    *get_note.Scenario
	create *create_note.Scenario
	update *update_note.Scenario
	del    *delete_note.Scenario
}

func (s *noteService) ListNotes(ctx context.Context, access models.Access, page models.Page) ([]models.Note, error) {
	return s.list.ListNotes(ctx, access, page)
}

func (s *noteService) SearchNotes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Note, error) {
	return s.search.SearchNotes(ctx, access, q)
}

func (s *noteService) GetNote(ctx context.Context, access models.Access, id models.ID) (models.Note, error) {
	return s.get.GetNote(ctx, access, id)
}

func (s *noteService) CreateNote(ctx context.Context, n models.Note) (models.Note, error) {
	return s.create.CreateNote(ctx, n)
}

func (s *noteService) UpdateNote(ctx context.Context, n models.Note) error {
	return s.update.UpdateNote(ctx, n)
}

func (s *noteService) DeleteNote(ctx context.Context, id models.ID) error {
	return s.del.DeleteNote(ctx, id)
}

// newNoteService собирает фасад на настоящем хранилище.
func newNoteService(t *testing.T, st *sqlstore.Store) *noteService {
	t.Helper()

	return &noteService{
		list:   list_notes.New(st),
		search: search_notes.New(st),
		get:    get_note.New(st),
		create: create_note.New(st, idgen.New()),
		update: update_note.New(st),
		del:    delete_note.New(st),
	}
}

// attachmentService — фасад httpapi.AttachmentService на настоящих сценариях
// (так же собран internal/app's attachmentService).
type attachmentService struct {
	list   *list_attachments.Scenario
	search *search_attachments.Scenario
	get    *get_attachment.Scenario
	create *create_attachment.Scenario
	update *update_attachment.Scenario
	del    *delete_attachment.Scenario
}

func (s *attachmentService) ListAttachments(ctx context.Context, access models.Access, page models.Page) ([]models.Attachment, error) {
	return s.list.ListAttachments(ctx, access, page)
}

func (s *attachmentService) SearchAttachments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Attachment, error) {
	return s.search.SearchAttachments(ctx, access, q)
}

func (s *attachmentService) GetAttachment(ctx context.Context, access models.Access, id models.ID) (models.Attachment, error) {
	return s.get.GetAttachment(ctx, access, id)
}

func (s *attachmentService) CreateAttachment(ctx context.Context, a models.Attachment) (models.Attachment, error) {
	return s.create.CreateAttachment(ctx, a)
}

func (s *attachmentService) UpdateAttachment(ctx context.Context, a models.Attachment) error {
	return s.update.UpdateAttachment(ctx, a)
}

func (s *attachmentService) DeleteAttachment(ctx context.Context, id models.ID) error {
	return s.del.DeleteAttachment(ctx, id)
}

// newAttachmentService собирает фасад на настоящем хранилище.
func newAttachmentService(t *testing.T, st *sqlstore.Store) *attachmentService {
	t.Helper()

	return &attachmentService{
		list:   list_attachments.New(st),
		search: search_attachments.New(st),
		get:    get_attachment.New(st),
		create: create_attachment.New(st, idgen.New()),
		update: update_attachment.New(st),
		del:    delete_attachment.New(st),
	}
}

// TestAdminDivisionsWithRealStore: сквозной путь «хранилище → сценарий → HTTP»
// на настоящей БД (так же собран internal/app): фильтры, окно после фильтра,
// parent_id, коды ошибок, прежнего пути нет.
func TestAdminDivisionsWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	root := models.ID("AD-11HFE865V215DE1CTEWH0AVNH9")
	ad1 := models.ID("AD-3N65R6PPG2X7R5JQC42EJ5E6RH")
	ad2 := models.ID("AD-7QAQPDH4AFAXRHEM3E2MSH4DMD")
	ad3 := models.ID("AD-7SX9G8FGVSQE8Z5379AV4RRXG0")
	missingParent := "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9" // валидный формат, не сохранён

	for _, d := range []models.AdministrativeDivision{
		{ID: root, Name: "Московская", Type: models.AdminDivisionGovernorate},
		{ID: ad1, Name: "Давыдово", Type: models.AdminDivisionSelo, ParentID: &root},
		{ID: ad2, Name: "Никифоровская", Type: models.AdminDivisionVolost, ParentID: &root,
			Variants: []string{"Никольское"}},
		{ID: ad3, Name: "Никифорово", Type: models.AdminDivisionDerevnya, ParentID: &root},
	} {
		if err := st.SaveAdministrativeDivision(t.Context(), &d); err != nil {
			t.Fatalf("save %s: %v", d.ID, err)
		}
	}

	h := httpapi.NewHandler(httpapi.Deps{Divisions: newDivisionService(t, st), DocsFS: fstest.MapFS{}})

	cases := []struct {
		target string
		code   int
		body   string // пусто — не сверять
	}{
		{"/api/admin-divisions?kind=settlement", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Давыдово","type":"selo","parent_id":%q,"sources":[]},`+
				`{"id":%q,"name":"Никифорово","type":"derevnya","parent_id":%q,"sources":[]}]`, ad1, root, ad3, root)},
		{"/api/admin-divisions?kind=settlement&limit=1&offset=1", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Никифорово","type":"derevnya","parent_id":%q,"sources":[]}]`, ad3, root)},
		{"/api/admin-divisions?type=governorate", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Московская","type":"governorate","parent_id":null,"sources":[]}]`, root)},
		{"/api/admin-divisions?type=castle", http.StatusUnprocessableEntity,
			`{"error":"type: неизвестный тип единицы деления \"castle\"","field":"type"}`},
		{"/api/admin-divisions?limit=x", http.StatusBadRequest,
			`{"error":"параметр limit: ожидалось целое число, получено \"x\""}`},
		{"/api/settlements", http.StatusNotFound, ""},
		{"/api/admin-divisions/search?q=давы", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Давыдово","type":"selo","parent_id":%q,"sources":[]}]`, ad1, root)},
		{"/api/admin-divisions/search?q=", http.StatusOK, `[]`},
		{"/api/admin-divisions/search?q=ник", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Никифоровская","type":"volost","parent_id":%q,"sources":[]},`+
				`{"id":%q,"name":"Никифорово","type":"derevnya","parent_id":%q,"sources":[]}]`, ad2, root, ad3, root)},
		{"/api/admin-divisions/search?q=никол", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Никифоровская","type":"volost","parent_id":%q,"sources":[]}]`, ad2, root)},
		{"/api/admin-divisions/search?q=ик", http.StatusOK, `[]`},
		{"/api/admin-divisions?parent_id=" + string(root), http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Давыдово","type":"selo","parent_id":%q,"sources":[]},`+
				`{"id":%q,"name":"Никифоровская","type":"volost","parent_id":%q,"sources":[]},`+
				`{"id":%q,"name":"Никифорово","type":"derevnya","parent_id":%q,"sources":[]}]`,
				ad1, root, ad2, root, ad3, root)},
		{"/api/admin-divisions?parent_id=" + missingParent, http.StatusNotFound, ""},
		{"/api/admin-divisions?parent_id=not-an-id", http.StatusUnprocessableEntity, ""},
	}

	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.target, nil))

		body := strings.TrimSpace(rec.Body.String())
		if rec.Code != c.code || (c.body != "" && body != c.body) {
			t.Errorf("GET %s = %d %s\n want %d %s", c.target, rec.Code, body, c.code, c.body)
		}
	}
}
