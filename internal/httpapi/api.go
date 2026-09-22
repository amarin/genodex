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
