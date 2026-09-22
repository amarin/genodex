# Хардening после программы auth: план

Не часть роадмапа A–D (`2026-09-22-auth-roadmap.md`) — отдельный проход по
двум рискам, принятым как «deploy blocker, не merge blocker» во время
финальных ревью этапов C/D (см. `docs/data-model/auth.md` §9 и §4).
Формат — как у прочих auth-планов. Исполняется в `main` подрядными
коммитами.

## Goal

Закрыть оба заранее задокументированных и сознательно отложенных риска
перед реальным деплоем:
1. TOCTOU-гонка на bootstrap-регистрации (`Service.Register` может создать
   двух владельцев при двух конкурентных первых запросах).
2. `Secure`-флаг cookie не включается за TLS-терминирующим реверс-прокси
   (`r.TLS` всегда `nil`, когда TLS снимает nginx/Caddy/Cloudflare, а не
   сам Go-процесс).

## Предпосылка: обсуждённые и отклонённые альтернативы

**По задаче 1** — рассматривались: (а) создавать владельца при старте
процесса со случайным/статичным паролем, печатать в консоль — отклонено:
ломается для GUI-запуска, где консоли нет; (б) выносить создание
владельца целиком в CLI/конфиг — отклонено: теряется осознанно выбранный
UX «зашёл в браузер — прошёл мастер регистрации» (решение этапа
брейнсторминга программы auth); (в) блокировка на уровне анонимной
сессии первого посетителя — отклонено: сама операция «захватить право
быть первым» — та же TOCTOU-гонка на уровень выше, плюс не</br>определено
поведение при незавершённом мастере регистрации. Выбрано: genodex —
однопроцессный сервис на одном соединении SQLite (`SetMaxOpenConns(1)`,
`README.md`: «Single-binary service») — гонка происходит между
горутинами ОДНОГО процесса, значит `sync.Mutex` в `auth.Service`
закрывает её полностью, без изменений в хранилище, CLI или UX.

**По задаче 2** — рассматривался отказ от cookie в пользу заголовка-
bearer-токена для веб-сессии (по аналогии с MCP API-токенами): снимает
вопрос `Secure` полностью и заодно упраздняет `requireCSRFHeader`, но
ценой реальной потери свойства (`HttpOnly` перестаёт защищать токен от
кражи через XSS — токен пришлось бы держать в доступном JS месте:
`localStorage`/переменная), Path-scoping refresh-cookie (сейчас физически
не покидает `/api/auth/refresh`) пришлось бы эмулировать руками, и объём
переделки — весь auth-стек этапов B–D, включая десятки существующих
тестов, ставящих cookie через `req.AddCookie`. Отклонено — размен не
оправдан при отсутствии другой причины уходить от cookie (нет
горизontального масштабирования, нет прокси/CDN, режущих cookie).
Выбрано: `-trust-proxy` флаг + `X-Forwarded-Proto` — тот же способ,
которым эту задачу решают Express (`trust proxy`), Django
(`SECURE_PROXY_SSL_HEADER`), Rails (`trusted_proxies`); не эксплуатируем
заголовок без явного включения (иначе недоверенный клиент мог бы
подделать его сам).

## Задача 1. Мьютекс на `Service.Register`

**Файлы:**
- Изменить: `internal/auth/service.go`, `internal/auth/service_test.go`,
  `docs/data-model/auth.md` (§9)

- [ ] Шаг 1.1. `internal/auth/service.go` — добавить импорт `"sync"`,
  добавить поле в структуру `Service`:
  ```go
  // Service — фасад над Store: вся бизнес-логика auth в одном месте (домен
  // маленький и плотно связанный — как сценарии usecases/*, но один пакет).
  type Service struct {
  	store Store
  	now   func() time.Time

  	// registerMu сериализует Register: между CountOwners и CreateOwner нет
  	// БД-транзакции (auth.SQLStore не использует транзакций), так что без
  	// мьютекса два конкурентных bootstrap-запроса на пустой БД оба видят
  	// «владельцев ноль» и оба проходят мимо проверки invite (TOCTOU,
  	// auth.md §9). genodex — однопроцессный сервис на одном соединении
  	// SQLite, поэтому мьютекс внутри процесса полностью закрывает гонку
  	// без изменений в хранилище; заодно делает избыточной (но не
  	// вредной — оставлена как есть) отдельную защиту от повторного
  	// использования invite в MarkInviteUsed (A2-fix-1) — тот же
  	// сериализованный путь закрывает и её.
  	registerMu sync.Mutex
  }
  ```
  (замени существующее объявление `type Service struct { store Store; now
  func() time.Time }` целиком на код выше — только добавляется поле и
  комментарий, `store`/`now` не меняются).

- [ ] Шаг 1.2. В том же файле — добавить блокировку первыми двумя строками
  тела `Register` (сигнатура и остальное тело — БЕЗ ИЗМЕНЕНИЙ, только
  вставка):
  ```go
  func (s *Service) Register(ctx context.Context, login, password string, invite *string) (AuthResult, error) {
  	s.registerMu.Lock()
  	defer s.registerMu.Unlock()

  	count, err := s.store.CountOwners(ctx)
  	if err != nil {
  		return AuthResult{}, err
  	}

  	var usedInvite *Invite

  	if count > 0 {
  		usedInvite, err = s.checkInvite(ctx, invite)
  		if err != nil {
  			return AuthResult{}, err
  		}
  	}

  	if _, err := s.store.GetOwnerByLogin(ctx, login); err == nil {
  		return AuthResult{}, ErrLoginTaken
  	} else if !errors.Is(err, ErrNotFound) {
  		return AuthResult{}, err
  	}

  	if err := validatePassword("password", password); err != nil {
  		return AuthResult{}, err
  	}

  	hash, err := hashPassword(password)
  	if err != nil {
  		return AuthResult{}, err
  	}

  	id, err := newID(KindOwner)
  	if err != nil {
  		return AuthResult{}, err
  	}

  	owner := Owner{ID: id, Login: login, PasswordHash: hash, CreatedAt: s.now()}
  	if err := owner.Validate(); err != nil {
  		return AuthResult{}, err
  	}

  	if err := s.store.CreateOwner(ctx, owner); err != nil {
  		return AuthResult{}, err
  	}

  	if usedInvite != nil {
  		if err := s.store.MarkInviteUsed(ctx, usedInvite.ID, owner.ID); err != nil {
  			return AuthResult{}, err
  		}
  	}

  	return s.newSession(ctx, owner.ID)
  }
  ```

- [ ] Шаг 1.3. `internal/auth/service_test.go` — добавить импорты `"fmt"`
  и `"sync"` в блок импортов (если их там ещё нет — `errors`/`strings`/
  `testing`/`time`/`context`/`models` уже есть, не дублируй), добавить
  новый тест (после `TestServiceRegisterDuplicateLoginFails`, рядом с
  остальными Register-тестами):
  ```go
  // TestServiceRegisterBootstrapRaceOnlyOneOwnerCreated: N конкурентных
  // bootstrap-регистраций (разные логины, без invite) на пустом сторе —
  // ровно одна проходит, остальные получают ErrInviteRequired (мьютекс
  // сериализует Register — второй и далее видят count>0). Гоняется под
  // -race: fakeStore использует обычные map без своей синхронизации —
  // если бы мьютекса не было или он был бы дырявым, конкурентный доступ к
  // map поймал бы race detector, а не только тест упал бы по количеству.
  func TestServiceRegisterBootstrapRaceOnlyOneOwnerCreated(t *testing.T) {
  	svc := New(newFakeStore())

  	const n = 10

  	var wg sync.WaitGroup

  	results := make([]error, n)

  	for i := 0; i < n; i++ {
  		wg.Add(1)

  		go func(i int) {
  			defer wg.Done()

  			_, err := svc.Register(context.Background(), fmt.Sprintf("owner%d", i), "password123", nil)
  			results[i] = err
  		}(i)
  	}

  	wg.Wait()

  	var succeeded, inviteRequired int

  	for _, err := range results {
  		switch {
  		case err == nil:
  			succeeded++
  		case errors.Is(err, ErrInviteRequired):
  			inviteRequired++
  		default:
  			t.Errorf("неожиданная ошибка: %v", err)
  		}
  	}

  	if succeeded != 1 {
  		t.Fatalf("succeeded = %d, ожидалась ровно 1 (гонка должна сериализоваться)", succeeded)
  	}

  	if inviteRequired != n-1 {
  		t.Fatalf("inviteRequired = %d, ожидалось %d", inviteRequired, n-1)
  	}
  }
  ```

- [ ] Шаг 1.4. `go test -race ./internal/auth/...` — обязательно с
  `-race` (не просто `go test`) — зелено, включая новый тест.

- [ ] Шаг 1.5. `docs/data-model/auth.md` §9 — найди два соседних пункта:
  первый начинается «Одноразовость invite гарантируется только на уровне
  одной строки хранилища...», второй сразу за ним — «Та же природа гонки
  — в самом bootstrap-пути...» (оба заканчиваются на «отложено на
  отдельный hardening-проход»). Замени ОБА этих пункта одним новым:
  ```
  - TOCTOU-гонка на `Service.Register` (bootstrap: два конкурентных первых
    запроса могли создать двух владельцев; invite: два конкурентных
    запроса с одним токеном могли оба пройти проверку) — закрыта
    `sync.Mutex` в `Service` (не транзакцией в БД): genodex — однопроцессный
    сервис на одном соединении SQLite, гонка была между горутинами одного
    процесса, мьютекс её полностью сериализует. `MarkInviteUsed`'s
    условный `UPDATE ... WHERE used_at IS NULL` (A2-fix-1) остался как
    есть — избыточен при мьютексе, но не вреден (defense-in-depth). См.
    `2026-09-22-auth-hardening.md`.
  ```

- [ ] Шаг 1.6. Рубеж: `gofmt -l .` пусто, `go build ./...`, `go vet
  ./...`, `go test -race ./...` (весь репозиторий, не только
  `internal/auth` — `-race` на всём прогоне дороже по времени, но это
  первый мьютекс во всей auth-подсистеме, стоит проверить отсутствие
  гонок нигде).

- [ ] Шаг 1.7. Коммит:
  `git add internal/auth/service.go internal/auth/service_test.go docs/data-model/auth.md`
  `fix(auth): sync.Mutex на Register — закрывает TOCTOU-гонку bootstrap и invite`.

## Задача 2. `-trust-proxy` флаг для `Secure`-cookie

**Файлы:**
- Изменить: `internal/httpapi/auth.go`, `internal/httpapi/api.go`,
  `internal/app/app.go`, `cmd/genodex/main.go`,
  `internal/httpapi/auth_test.go`, `internal/httpapi/auth_service_test.go`,
  `internal/httpapi/write_store_test.go`, `docs/data-model/auth.md` (§4),
  `README.md`

- [ ] Шаг 2.1. `internal/httpapi/auth.go` — заменить `NewAuthHandler` и
  `registerAuthRoutes`:
  ```go
  // NewAuthHandler строит обработчики /api/auth/*, обёрнутые resolveAccess и
  // requireCSRFHeader — используется юнит-тестами этого пакета напрямую.
  // Реальное приложение монтирует весь /api/ через NewAPIHandler (api.go),
  // который вызывает registerAuthRoutes без повторного оборачивания.
  // trustProxy — см. isSecureRequest.
  func NewAuthHandler(auth AuthService, trustProxy bool) http.Handler {
  	mux := http.NewServeMux()
  	registerAuthRoutes(mux, auth, trustProxy)

  	return requireCSRFHeader(resolveAccess(auth)(mux))
  }

  // registerAuthRoutes регистрирует маршруты /api/auth/* на переданном mux.
  func registerAuthRoutes(mux *http.ServeMux, auth AuthService, trustProxy bool) {
  	mux.HandleFunc("GET /api/auth/status", handleAuthStatus(auth))
  	mux.HandleFunc("GET /api/auth/session", handleAuthSession(auth))
  	mux.HandleFunc("POST /api/auth/register", handleAuthRegister(auth, trustProxy))
  	mux.HandleFunc("POST /api/auth/login", handleAuthLogin(auth, trustProxy))
  	mux.HandleFunc("POST /api/auth/logout", handleAuthLogout(auth, trustProxy))
  	mux.HandleFunc("POST /api/auth/refresh", handleAuthRefresh(auth, trustProxy))
  	mux.HandleFunc("POST /api/auth/password", handleAuthPassword(auth, trustProxy))
  	mux.HandleFunc("POST /api/auth/invites", handleAuthCreateInvite(auth))
  	mux.HandleFunc("POST /api/auth/tokens", handleAuthCreateToken(auth))
  	mux.HandleFunc("GET /api/auth/tokens", handleAuthListTokens(auth))
  	mux.HandleFunc("DELETE /api/auth/tokens/{id}", handleAuthRevokeToken(auth))
  }
  ```

- [ ] Шаг 2.2. В том же файле — заменить `setSessionCookies`/
  `clearSessionCookies`, добавить `isSecureRequest` перед ними:
  ```go
  // isSecureRequest решает, ставить ли Secure на cookie: TLS терминирует сам
  // процесс (r.TLS != nil), либо явно включённое доверие к обратному прокси
  // (-trust-proxy) видит X-Forwarded-Proto: https. Без явного включения
  // заголовок никогда не учитывается: недоверенный клиент мог бы подделать
  // его сам, а без реального прокси перед этим процессом заголовку верить
  // нельзя (auth.md §4).
  func isSecureRequest(r *http.Request, trustProxy bool) bool {
  	if r.TLS != nil {
  		return true
  	}

  	return trustProxy && r.Header.Get("X-Forwarded-Proto") == "https"
  }

  // setSessionCookies выставляет пару access/refresh cookie (auth.md §4,
  // решение 8): access — Path=/, refresh — Path=/api/auth/refresh (уже, чем
  // сайт целиком). Secure — см. isSecureRequest.
  func setSessionCookies(w http.ResponseWriter, r *http.Request, res authpkg.AuthResult, trustProxy bool) {
  	secure := isSecureRequest(r, trustProxy)

  	http.SetCookie(w, &http.Cookie{
  		Name: accessCookieName, Value: res.AccessToken, Path: "/",
  		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
  		Expires: res.AccessExpiresAt,
  	})
  	http.SetCookie(w, &http.Cookie{
  		Name: refreshCookieName, Value: res.RefreshToken, Path: "/api/auth/refresh",
  		HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode,
  		Expires: res.RefreshExpiresAt,
  	})
  }

  // clearSessionCookies стирает обе cookie (логаут, смена пароля, просроченный
  // refresh). Secure — см. isSecureRequest.
  func clearSessionCookies(w http.ResponseWriter, r *http.Request, trustProxy bool) {
  	secure := isSecureRequest(r, trustProxy)

  	http.SetCookie(w, &http.Cookie{
  		Name: accessCookieName, Value: "", Path: "/",
  		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1,
  	})
  	http.SetCookie(w, &http.Cookie{
  		Name: refreshCookieName, Value: "", Path: "/api/auth/refresh",
  		HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, MaxAge: -1,
  	})
  }
  ```

- [ ] Шаг 2.3. В том же файле — заменить сигнатуры и тела 5 хендлеров,
  добавив параметр `trustProxy bool` и прокидывая его в
  `setSessionCookies`/`clearSessionCookies` (остальные 6 хендлеров —
  `handleAuthStatus`, `handleAuthSession`, `handleAuthCreateInvite`,
  `handleAuthCreateToken`, `handleAuthListTokens`, `handleAuthRevokeToken`
  — НЕ трогать, они не выставляют cookie):

  ```go
  // handleAuthRegister — POST /api/auth/register?invite=<raw>; тело
  // {login, password}. bootstrap (нет владельцев) — invite не нужен.
  func handleAuthRegister(auth AuthService, trustProxy bool) http.HandlerFunc {
  	return func(w http.ResponseWriter, r *http.Request) {
  		var in transport.RegisterRequest
  		if err := decodeJSON(r, &in); err != nil {
  			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

  			return
  		}

  		var invite *string
  		if raw := r.URL.Query().Get("invite"); raw != "" {
  			invite = &raw
  		}

  		res, err := auth.Register(r.Context(), in.Login, in.Password, invite)
  		if err != nil {
  			writeAuthError(w, err)

  			return
  		}

  		setSessionCookies(w, r, res, trustProxy)
  		// Логин в ответе — из тела запроса, не из GetOwner: Service.Register не
  		// нормализует login (без trim/case-fold) нигде, так что эхо входа
  		// вызывающего корректно само по себе и экономит лишнее чтение БД. Не
  		// «чинить» на GetOwner ради единообразия с handleAuthSession/Refresh.
  		writeJSON(w, http.StatusCreated, transport.AuthSession{Login: in.Login})
  	}
  }

  // handleAuthLogin — POST /api/auth/login; тело {login, password}.
  func handleAuthLogin(auth AuthService, trustProxy bool) http.HandlerFunc {
  	return func(w http.ResponseWriter, r *http.Request) {
  		var in transport.LoginRequest
  		if err := decodeJSON(r, &in); err != nil {
  			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

  			return
  		}

  		res, err := auth.Login(r.Context(), in.Login, in.Password)
  		if err != nil {
  			writeAuthError(w, err)

  			return
  		}

  		setSessionCookies(w, r, res, trustProxy)
  		// См. комментарий в handleAuthRegister: логин — эхо входа, GetOwner тут
  		// не нужен.
  		writeJSON(w, http.StatusOK, transport.AuthSession{Login: in.Login})
  	}
  }

  // handleAuthLogout — POST /api/auth/logout: требует Full.
  func handleAuthLogout(auth AuthService, trustProxy bool) http.HandlerFunc {
  	return func(w http.ResponseWriter, r *http.Request) {
  		if _, ok := requireFull(w, r); !ok {
  			return
  		}

  		if c, err := r.Cookie(accessCookieName); err == nil {
  			if err := auth.Logout(r.Context(), c.Value); err != nil {
  				writeAuthError(w, err)

  				return
  			}
  		}

  		clearSessionCookies(w, r, trustProxy)
  		w.WriteHeader(http.StatusNoContent)
  	}
  }

  // handleAuthRefresh — POST /api/auth/refresh: по refresh-cookie (не по
  // access — доступ проверяется отдельно от общего resolveAccess).
  func handleAuthRefresh(auth AuthService, trustProxy bool) http.HandlerFunc {
  	return func(w http.ResponseWriter, r *http.Request) {
  		c, err := r.Cookie(refreshCookieName)
  		if err != nil {
  			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "нет refresh-сессии"})

  			return
  		}

  		res, err := auth.Refresh(r.Context(), c.Value)
  		if err != nil {
  			clearSessionCookies(w, r, trustProxy)
  			writeAuthError(w, err)

  			return
  		}

  		// Cookies ставим сразу после успешной ротации, ДО GetOwner: Refresh уже
  		// необратимо заменил сессию в БД (старый refresh теперь мёртв), поэтому
  		// если GetOwner ниже упадёт, браузер всё равно должен получить новые
  		// access/refresh — иначе он остался бы с мёртвой cookie без пути
  		// восстановления, кроме повторного логина.
  		setSessionCookies(w, r, res, trustProxy)

  		owner, err := auth.GetOwner(r.Context(), res.OwnerID)
  		if err != nil {
  			writeAuthError(w, err)

  			return
  		}

  		writeJSON(w, http.StatusOK, transport.AuthSessionFromOwner(owner))
  	}
  }

  // handleAuthPassword — POST /api/auth/password: требует Full; тело
  // {current_password, new_password}. Успех гасит ВСЕ сессии владельца
  // (эффект Service.ChangePassword, этап A2) включая сессию самого вызывающего
  // — обработчик поэтому сам чистит его cookies и отвечает 204 без тела; веб
  // (этап D) обязан отправить пользователя на /login.
  func handleAuthPassword(auth AuthService, trustProxy bool) http.HandlerFunc {
  	return func(w http.ResponseWriter, r *http.Request) {
  		ownerID, ok := requireFull(w, r)
  		if !ok {
  			return
  		}

  		var in transport.PasswordChangeRequest
  		if err := decodeJSON(r, &in); err != nil {
  			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

  			return
  		}

  		if err := auth.ChangePassword(r.Context(), ownerID, in.CurrentPassword, in.NewPassword); err != nil {
  			writeAuthError(w, err)

  			return
  		}

  		clearSessionCookies(w, r, trustProxy)
  		w.WriteHeader(http.StatusNoContent)
  	}
  }
  ```

- [ ] Шаг 2.4. `internal/httpapi/api.go` — заменить `NewAPIHandler`:
  ```go
  package httpapi

  import (
  	"io/fs"
  	"net/http"
  )

  // NewAPIHandler — единая точка входа /api: маршруты делений, документации и
  // auth на одном mux, обёрнутые ОДИН раз resolveAccess + requireCSRFHeader
  // (auth.md §4 — исходный замысел дизайна: оба миддлвари вокруг всего /api/,
  // не только /api/auth/*). Это то, что реально монтирует internal/app
  // (этап C). trustProxy — см. isSecureRequest (auth.go), включается флагом
  // -trust-proxy. NewHandler и NewAuthHandler остаются отдельно для
  // существующих юнит-тестов пакета, не зависящих от auth.
  func NewAPIHandler(divisions DivisionService, auth AuthService, docsFS fs.FS, trustProxy bool) http.Handler {
  	mux := http.NewServeMux()
  	registerDivisionRoutes(mux, divisions, docsFS)
  	registerAuthRoutes(mux, auth, trustProxy)

  	return requireCSRFHeader(resolveAccess(auth)(mux))
  }
  ```

- [ ] Шаг 2.5. `internal/app/app.go` — в `Config` добавить поле, в `New`
  прокинуть его в `NewAPIHandler`:
  ```go
  // Config — параметры запуска приложения.
  type Config struct {
  	DataDir    string
  	Port       int
  	WebMode    string
  	TrustProxy bool
  }
  ```
  и заменить строку монтирования `/api/`:
  ```go
  	mux.Handle("/api/", httpapi.NewAPIHandler(divisions, authService, genodex.DocsFS(cfg.WebMode), cfg.TrustProxy))
  ```
  (это единственная строка в файле, которую нужно поменять помимо
  структуры `Config` — остальной `app.go`, включая `divisionService` и
  `Run`, не трогать).

- [ ] Шаг 2.6. `cmd/genodex/main.go`, функция `runServe` — добавить флаг и
  прокинуть его в `app.Config`:
  ```go
  	fs := newFlagSet("serve")
  	_ = dataDirFlag(fs)
  	port := fs.Int("p", 9000, "HTTP server port")
  	webMode := fs.String("web", web.ModeProd, "web assets mode: prod (embedded) or dev (from disk)")
  	trustProxy := fs.Bool("trust-proxy", false,
  		"trust X-Forwarded-Proto from a reverse proxy for the cookie Secure flag (enable only behind a TLS-terminating proxy you control)")
  	if err := fs.Parse(args); err != nil {
  		return err
  	}
  	if fs.NArg() > 0 {
  		return fmt.Errorf("unknown argument %q", fs.Arg(0))
  	}

  	a, err := app.New(app.Config{
  		DataDir:    chooseDataDir(fs, preDataDir),
  		Port:       *port,
  		WebMode:    *webMode,
  		TrustProxy: *trustProxy,
  	})
  ```
  (замени соответствующий кусок `runServe` — от объявления `fs :=
  newFlagSet("serve")` до вызова `app.New(...)` включительно; остальная
  часть `runServe` после этого вызова, и весь остальной файл, не менять).

- [ ] Шаг 2.7. `internal/httpapi/auth_test.go` — механическая правка: во
  ВСЕХ вызовах `NewAuthHandler(svc)`/`NewAuthHandler(svc2)` (их 26 —
  `svc`/`svc2` разные имена переменной, шаблон вызова один) добавить
  `false` вторым аргументом — например было `h := NewAuthHandler(svc)`,
  стало `h := NewAuthHandler(svc, false)`. Компилятор — чек-лист (`go
  build`/`go vet` укажут каждое место с ошибкой «not enough arguments»).

- [ ] Шаг 2.8. В том же файле — добавить новый тест (в конец файла):
  ```go
  // TestSetSessionCookiesSecureFlag: Secure зависит от TLS-терминации ИЛИ
  // явно включённого доверия к прокси (-trust-proxy) — заголовок
  // X-Forwarded-Proto сам по себе, без включения, ни на что не влияет
  // (недоверенный клиент мог бы его подделать).
  func TestSetSessionCookiesSecureFlag(t *testing.T) {
  	cases := []struct {
  		name       string
  		trustProxy bool
  		forwarded  string
  		want       bool
  	}{
  		{"trustProxy=false, заголовка нет — не secure", false, "", false},
  		{"trustProxy=false, заголовок лжёт https — не secure (не доверяем без включения)", false, "https", false},
  		{"trustProxy=true, заголовок https — secure", true, "https", true},
  		{"trustProxy=true, заголовок http — не secure", true, "http", false},
  		{"trustProxy=true, заголовка нет — не secure", true, "", false},
  	}

  	for _, c := range cases {
  		t.Run(c.name, func(t *testing.T) {
  			svc := &fakeAuthService{registerResult: authpkg.AuthResult{
  				OwnerID: "OW-1", AccessToken: "acc", RefreshToken: "ref",
  			}}
  			h := NewAuthHandler(svc, c.trustProxy)

  			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"login":"first","password":"password123"}`))
  			req.Header.Set("X-Requested-With", "genodex")
  			if c.forwarded != "" {
  				req.Header.Set("X-Forwarded-Proto", c.forwarded)
  			}

  			rec := httptest.NewRecorder()
  			h.ServeHTTP(rec, req)

  			var accessCookie *http.Cookie
  			for _, ck := range rec.Result().Cookies() {
  				if ck.Name == accessCookieName {
  					accessCookie = ck
  				}
  			}
  			if accessCookie == nil {
  				t.Fatal("access cookie не выставлена")
  			}

  			if accessCookie.Secure != c.want {
  				t.Errorf("Secure = %v, ожидалось %v", accessCookie.Secure, c.want)
  			}
  		})
  	}
  }
  ```

- [ ] Шаг 2.9. `internal/httpapi/auth_service_test.go` — строка 24
  (`return httpapi.NewAuthHandler(auth.New(auth.NewSQLStore(st.DB())))`)
  — добавить `, false` перед закрывающей скобкой `NewAuthHandler`:
  `return httpapi.NewAuthHandler(auth.New(auth.NewSQLStore(st.DB())), false)`.

- [ ] Шаг 2.10. `internal/httpapi/write_store_test.go` — оба вызова
  `httpapi.NewAPIHandler(newDivisionService(t, st), authSvc,
  fstest.MapFS{})` (строки 32 и 118) — добавить `, false` четвёртым
  аргументом: `httpapi.NewAPIHandler(newDivisionService(t, st), authSvc,
  fstest.MapFS{}, false)`.

- [ ] Шаг 2.11. `go build ./...` — провалится, пока не поправлены ВСЕ
  вызовы из шагов 2.7/2.9/2.10 — используй ошибки компиляции как
  чек-лист. `gofmt -w internal/httpapi/`. `go test ./internal/httpapi/...`
  — зелено, включая новый `TestSetSessionCookiesSecureFlag` (5
  подтестов).

- [ ] Шаг 2.12. `docs/data-model/auth.md` §4 — найди абзац «Известный
  пробел: `setSessionCookies`/`clearSessionCookies` ставят флаг `Secure`
  только когда `r.TLS != nil`...» (заканчивается «закрывает тот, кто
  подключает сервис к реальному деплою за прокси»). Замени его на:
  ```
  `Secure` учитывает либо `r.TLS != nil` (этот процесс сам терминирует
  TLS), либо, при явно включённом флаге `-trust-proxy`, заголовок
  `X-Forwarded-Proto: https` от реверс-прокси (`isSecureRequest`,
  `internal/httpapi/auth.go`) — без включения заголовок не учитывается
  вообще, иначе недоверенный клиент мог бы подделать его сам. Включать
  `-trust-proxy` можно только за прокси, которому доверяете (он либо сам
  зачищает/переписывает входящий `X-Forwarded-Proto`, либо стоит первым
  на пути к процессу и клиент до него не дотягивается напрямую) — иначе
  клиент, обратившийся к genodex в обход прокси, сможет выставить cookie
  с `Secure` на самом деле не-HTTPS соединении, что не эксплуатируется
  атакующим (браузеры сами не отправляют `Secure`-cookie не по HTTPS), но
  ломает сессии таких запросов. См. `2026-09-22-auth-hardening.md`.
  ```

- [ ] Шаг 2.13. `README.md` — в таблицу флагов добавить строку:
  ```
  | `-trust-proxy` | `false` | Trust `X-Forwarded-Proto` from a reverse proxy for the cookie `Secure` flag (enable only behind a TLS-terminating proxy you control) |
  ```
  (после строки `-web`).

- [ ] Шаг 2.14. Рубеж: `gofmt -l .` пусто, `go build ./...`, `go vet
  ./...`, `go test ./...` — весь репозиторий зелёный.

- [ ] Шаг 2.15. Коммит:
  `git add internal/httpapi/auth.go internal/httpapi/api.go internal/app/app.go cmd/genodex/main.go internal/httpapi/auth_test.go internal/httpapi/auth_service_test.go internal/httpapi/write_store_test.go docs/data-model/auth.md README.md`
  `feat(httpapi): -trust-proxy — Secure-cookie за TLS-терминирующим прокси`.

## Рубеж прохода

- Конкурентные bootstrap/invite-регистрации сериализованы, доказано
  тестом под `-race`.
- `Secure` учитывает `X-Forwarded-Proto` только при явном `-trust-proxy`.
- `docs/data-model/auth.md` §4/§9 обновлены, README.md — новый флаг.
- `go test -race ./...` зелёный по всему репозиторию.

## Коммиты

1. `fix(auth): sync.Mutex на Register — закрывает TOCTOU-гонку bootstrap и invite`
2. `feat(httpapi): -trust-proxy — Secure-cookie за TLS-терминирующим прокси`
