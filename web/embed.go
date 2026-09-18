package web

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

const (
	// ModeProd отдаёт собранный фронтенд из бинарника.
	ModeProd = "prod"
	// ModeDev читает собранный фронтенд с диска (web/dist).
	ModeDev = "dev"
)

// FS возвращает файловую систему собранного фронтенда.
func FS(mode string) fs.FS {
	if mode == ModeDev {
		return os.DirFS("web/dist")
	}
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err) // web/dist пустой или отсутствует: запустите npm run build
	}
	return sub
}

// StaticHandler отдаёт файлы собранного фронтенда (под /static/).
func StaticHandler(mode string) http.Handler {
	return http.FileServerFS(FS(mode))
}

// SPAHandler отдаёт index.html в корне и для клиентских роутов.
func SPAHandler(mode string) http.Handler {
	fsys := FS(mode)
	index, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		panic(err) // собранный фронтенд не найден: запустите npm run build
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(fsys, name); err != nil {
			name = "index.html"
		}

		if name == "index.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(index)
			return
		}

		http.ServeFileFS(w, r, fsys, name)
	})
}
