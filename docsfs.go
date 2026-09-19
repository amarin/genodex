package genodex

import (
	"embed"
	"io/fs"
	"os"

	"github.com/amarin/genodex/web"
)

//go:embed all:docs
var docsFS embed.FS

// DocsFS возвращает файловую систему документации (docs/).
// dev — с диска, prod — встроенная в бинарник.
// Режимы совпадают с web.ModeProd/web.ModeDev.
func DocsFS(mode string) fs.FS {
	if mode == web.ModeDev {
		return os.DirFS("docs")
	}
	sub, err := fs.Sub(docsFS, "docs")
	if err != nil {
		panic(err) // папка docs отсутствует в embed
	}
	return sub
}
