package models

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestNoJSONTags фиксирует границу пакетов: домен о JSON не знает,
// форма на проводе определяется в internal/transport.
func TestNoJSONTags(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, e.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if field, ok := n.(*ast.Field); ok && field.Tag != nil && strings.Contains(field.Tag.Value, "json:") {
				t.Errorf("%s: JSON-тег в домене: %s", fset.Position(field.Pos()), field.Tag.Value)
			}
			return true
		})
	}
}
