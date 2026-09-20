package httpapi

import (
	"io/fs"
	"net/http"
	"sort"
	"strings"
)

type docFile struct {
	Path  string `json:"path"`
	Title string `json:"title"`
}

func listDocs(fsys fs.FS) ([]docFile, error) {
	var files []docFile
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := e.Name()
		title := docTitle(fsys, path)
		files = append(files, docFile{Path: path, Title: title})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func walkDocs(fsys fs.FS) ([]docFile, error) {
	var files []docFile
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		title := docTitle(fsys, path)
		files = append(files, docFile{Path: path, Title: title})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func docTitle(fsys fs.FS, path string) string {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return path
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return strings.TrimSuffix(path, ".md")
}

func handleDocList(fsys fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := listDocs(fsys)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"files": files})
	}
}

// validDocPath проверяет путь документа: только .md, без выхода за пределы дерева.
func validDocPath(path string) bool {
	if !strings.HasSuffix(path, ".md") {
		return false
	}
	return fs.ValidPath(path)
}

func handleDocContent(fsys fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		if !validDocPath(path) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
			return
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write(data)
	}
}
