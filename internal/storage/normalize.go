package storage

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Normalize приводит строку к канонической форме для поиска по кириллице:
// нижний регистр, ё→е, NFD и удаление комбинирующихся символов (диакритики).
// Применяется и при записи search-индекса, и при поисковом запросе.
func Normalize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "ё", "е")
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if unicode.IsMark(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
