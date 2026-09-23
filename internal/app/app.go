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
	create_division "github.com/amarin/genodex/internal/usecases/create_division"
	create_surname "github.com/amarin/genodex/internal/usecases/create_surname"
	delete_division "github.com/amarin/genodex/internal/usecases/delete_division"
	delete_surname "github.com/amarin/genodex/internal/usecases/delete_surname"
	get_division "github.com/amarin/genodex/internal/usecases/get_division"
	get_surname "github.com/amarin/genodex/internal/usecases/get_surname"
	list_divisions "github.com/amarin/genodex/internal/usecases/list_divisions"
	list_surnames "github.com/amarin/genodex/internal/usecases/list_surnames"
	search_divisions "github.com/amarin/genodex/internal/usecases/search_divisions"
	search_surnames "github.com/amarin/genodex/internal/usecases/search_surnames"
	update_division "github.com/amarin/genodex/internal/usecases/update_division"
	update_surname "github.com/amarin/genodex/internal/usecases/update_surname"
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

var (
	_ httpapi.DivisionService = (*divisionService)(nil)
	_ mcp.DivisionService     = (*divisionService)(nil)
	_ httpapi.SurnameService  = (*surnameService)(nil)
	_ mcp.SurnameService      = (*surnameService)(nil)
	_ httpapi.AuthService     = (*auth.Service)(nil)
	_ mcp.TokenResolver       = (*auth.Service)(nil)
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

	// auth-хранилище — на том же соединении, что и общий store (см.
	// sqlstore.Store.DB), файл БД один и тот же (internal/storage/schema_auth.go).
	authService := auth.New(auth.NewSQLStore(st.DB()))

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.RequireAPIToken(authService)(server.NewStreamableHTTPServer(
		mcp.NewServer(mcp.Deps{Divisions: divisions, Surnames: surnames}),
	)))
	mux.Handle("/api/", httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  divisions,
		Surnames:   surnames,
		Auth:       authService,
		DocsFS:     genodex.DocsFS(cfg.WebMode),
		TrustProxy: cfg.TrustProxy,
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
