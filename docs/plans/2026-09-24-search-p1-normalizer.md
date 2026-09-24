# Поиск по базе — подпроект P1 (нормализатор и словари): план

> **Для агента-исполнителя:** ОБЯЗАТЕЛЬНЫЙ ПОДНАВЫК: superpowers:subagent-driven-development (рекомендуется) или superpowers:executing-plans. Шаги — чекбоксы (`- [ ]`).

**Goal:** Общий нормализатор текста (дореформенная орфография, токены, сокращения, леммы через gomorphy, стоп-слова) и реестр словарей (встроенный базовый, встроенные сокращения, папка словарей, включение/выключение, версия набора) — фундамент для индекса P2, NER (E9), дублей (E10) и сопоставления (F5).

**Architecture:** Три новых пакета. `internal/textnorm` — чистые функции без словарей: орфография, токенизатор с байтовыми смещениями, варианты дореформенных окончаний. `internal/dicts` — реестр словарей gomorphy: встроенный базовый (`//go:embed`, собирается `go generate`), встроенный стартовый словарь сокращений (TSV), файлы `<kind>.<name>.dat|tsv` из папки; состояние «включён» — в таблице `dictionaries` той же SQLite; неизменяемый снимок включённых словарей подменяется атомарно. `internal/normalizer` — разбор текста и запроса в термины с леммами и признаками поверх `dicts` (через интерфейс `Dictionaries`). Контракты `/api` и MCP не меняются; добавляются CLI `genodex analyze` и `genodex dicts`.

**Tech Stack:** Go 1.27.1, `github.com/amarin/gomorphy` (≥ версии с `morphology.OpenBytes` и `Reading.Predicted`), `golang.org/x/text/unicode/norm`, `modernc.org/sqlite`, `go.uber.org/mock`.

**Spec:** [docs/search/normalization.md](../search/normalization.md) (слои, словари, запросы в gomorphy); обзор — [docs/search/index.md](../search/index.md).

## Global Constraints

- Go поднимается до **1.27.1** (`go.mod`: `go 1.27.1`) — этого требует gomorphy.
- Исходный текст не меняется; нормализация — только для индекса и сравнения. Токен хранит байтовые смещения `Start`/`End` в исходном тексте.
- Не-кириллические токены не индексируются; исключение — латинская `i`/`I` рядом с кириллицей (набор «Iоаннъ»), она читается как `і`.
- `й` сохраняется (`Андрей`, `Покровский`); остальные надстрочные знаки (титла, ударения) отбрасываются.
- Стоп-слова — по тегу части речи gomorphy (`PREP`, `CONJ`, `PRCL`, `INTJ`), если **все** точные разборы служебные; списка стоп-слов в коде нет.
- Предсказанные разборы (`Predicted`) используются, только если нет ни одного точного; предсказания словарей сокращений не используются никогда.
- Неизвестное слово и число — лемма равна форме, признак `FlagUnknown`.
- Имена файлов словарей: `<kind>.<name>.dat` (бинарный gomorphy) или `<kind>.<name>.tsv` (`lemma<TAB>wordform[<TAB>tags]`); `<kind>` ∈ `base, abbrev, surname, given, patronymic, toponym`, иначе — `custom`. Имя словаря — имя файла без расширения.
- Файл `base.opencorpora.*` в папке заменяет встроенный базовый словарь; остальные файлы добавляются после встроенных, по алфавиту.
- Состояние «включён/выключен» — в таблице `dictionaries(name, enabled)`; словарь без записи включён. Порядок словарей в P1 не настраивается.
- Битый файл словаря не валит запуск: попадает в список с текстом ошибки и не участвует в разборе («никакого молчаливого отбрасывания»).
- Сборка по умолчанию требует `internal/dicts/base/base.opencorpora.dat` (`go generate ./internal/dicts/`, нужен интернет — как `web/dist` для фронта); сборка с тегом `nobasedict` обходится без него — тогда базовый словарь можно скачать позже `genodex dicts fetch`.
- Комментарии, тексты ошибок, вывод CLI — на русском. Одна структура — один файл (`DEVELOPER-PREFERENCES.md`). Межпакетные зависимости — интерфейсы в `deps.go` с `//go:generate mockgen`.
- Проверка рубежа: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`, а также `go vet -tags nobasedict ./...` и `go test -tags nobasedict ./...`.

---

### Task 0 (владелец, до исполнения): доработки gomorphy

Выполняет владелец в `github.com/amarin/gomorphy`, исполнитель плана не трогает библиотеку. План опирается ровно на эти два API:

1. `func OpenBytes(data []byte) (*Dictionary, error)` — как `Open`, но поверх переданного буфера (формат GMOR, тот же, что пишет `SaveTo`); буфер не копируется и должен жить, пока жив словарь; `Close` — no-op.
2. `Reading.Predicted bool` — `true` у разборов, полученных предсказанием по окончанию (`predict`), `false` у словарных (`exact`).

Выпускается версия библиотеки (ожидается `v1.2.0`). Исполнитель подставляет фактическую версию в `go get` задачи 3. Задачи 1–2 от gomorphy не зависят и могут идти до выпуска.

---

### Task 1: `textnorm` — орфография

**Files:**
- Create: `internal/textnorm/orthography.go`
- Test: `internal/textnorm/orthography_test.go`

**Interfaces:**
- Produces: `textnorm.RulesVersion` (const string), `textnorm.Orthography(s string) string`, `textnorm.NormalizeWord(w string) string`.

- [ ] **Step 1: Тест**

```go
package textnorm

import "testing"

// TestOrthography: регистр, дореформенные буквы, надстрочные знаки; й сохраняется;
// конечный ъ остаётся (его снимает NormalizeWord).
func TestOrthography(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Ивановъ", "ивановъ"},
		{"Покровскій", "покровский"},
		{"Ѳеодоръ", "феодоръ"},
		{"Евдокія", "евдокия"},
		{"Iоаннъ", "иоаннъ"},                 // латинская I в кириллическом слове
		{"Бѣлёвъ", "белевъ"},
		{"Бг҃ъ", "бгъ"},                 // титло
		{"Андрей", "андрей"},
		{"Андрей", "андрей"},           // й, набранное разложенным
		{"Кузнецо́в", "кузнецов"},       // ударение
		{"Ѡ ѯ ѱ ꙋ ѧ ѵ", "о кс пс у я и"},
		{"кр‐нин", "кр-нин"},            // U+2010 hyphen
	}
	for _, c := range cases {
		if got := Orthography(c.in); got != c.want {
			t.Errorf("Orthography(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestNormalizeWord: плюс отбрасывание конечного ъ у каждой части составного слова.
func TestNormalizeWord(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Ивановъ", "иванов"},
		{"Санктъ-Петербургъ", "санкт-петербург"},
		{"Бг҃ъ", "бг"},
		{"объявленіе", "объявление"}, // ъ внутри слова остаётся
	}
	for _, c := range cases {
		if got := NormalizeWord(c.in); got != c.want {
			t.Errorf("NormalizeWord(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./internal/textnorm/ -count=1`
Expected: FAIL — `undefined: Orthography`.

- [ ] **Step 3: Реализация**

```go
// Package textnorm — правила приведения текста к виду для индекса и
// сравнения: орфография, токены, варианты дореформенных окончаний. Без
// словарей; исходный текст не меняется (docs/search/normalization.md).
package textnorm

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// RulesVersion — версия правил пакета. Меняется при любом изменении таблиц
// и алгоритмов: формы в поисковом индексе зависят от неё (P2 перестраивает
// индекс при несовпадении).
const RulesVersion = "1"

// letterMap — замены дореформенных и церковнославянских букв (после нижнего
// регистра и разложения). ё, ї, ѷ отдельно не нужны: разложение даёт е, і, ѵ
// плюс знак, который отбрасывается.
var letterMap = map[rune]string{
	'ѣ': "е", 'і': "и", 'ѵ': "и", 'i': "и",
	'ѳ': "ф", 'ѡ': "о", 'ꙋ': "у", 'ѹ': "у", 'ѧ': "я", 'ѯ': "кс", 'ѱ': "пс",
	'‐': "-",
}

// Orthography приводит строку к современной орфографии для сравнения:
// нижний регистр, без надстрочных знаков (кроме й), дореформенные буквы — по
// letterMap. Конечный ъ не трогает — это правило слова (NormalizeWord).
func Orthography(s string) string {
	var b strings.Builder

	for _, r := range norm.NFC.String(strings.ToLower(s)) {
		switch {
		case r == 'й':
			b.WriteRune(r)
		case unicode.IsMark(r):
		default:
			for _, d := range norm.NFD.String(string(r)) {
				if unicode.IsMark(d) {
					continue
				}

				if rep, ok := letterMap[d]; ok {
					b.WriteString(rep)

					continue
				}

				b.WriteRune(d)
			}
		}
	}

	return b.String()
}

// NormalizeWord — форма слова для индекса: Orthography плюс отбрасывание
// конечного ъ у каждой части составного слова («санктъ-петербургъ» →
// «санкт-петербург»).
func NormalizeWord(w string) string {
	parts := strings.Split(Orthography(w), "-")
	for i, p := range parts {
		parts[i] = strings.TrimSuffix(p, "ъ")
	}

	return strings.Join(parts, "-")
}
```

- [ ] **Step 4: Тесты проходят**

Run: `gofmt -l internal/textnorm && go vet ./internal/textnorm/ && go test ./internal/textnorm/ -count=1`
Expected: gofmt пусто, `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/textnorm/
git commit -m "feat(textnorm): орфография для индекса — дореформенные буквы, надстрочные знаки, конечный ъ"
```

---

### Task 2: `textnorm` — токенизатор и дореформенные окончания

**Files:**
- Create: `internal/textnorm/token.go`, `internal/textnorm/tokenize.go`, `internal/textnorm/reform.go`
- Test: `internal/textnorm/tokenize_test.go`, `internal/textnorm/reform_test.go`

**Interfaces:**
- Consumes: `NormalizeWord` (задача 1).
- Produces: `textnorm.Token{Raw, Form string; Start, End int; Dotted, Number bool}`, `textnorm.Tokenize(text string) []Token`, `textnorm.ReformVariants(form string) []string`.

- [ ] **Step 1: Тесты**

`internal/textnorm/tokenize_test.go`:

```go
package textnorm

import (
	"strings"
	"testing"
)

// TestTokenize: слова с дефисом, точка-признак сокращения, числа отдельно
// от букв, смещения указывают в исходный текст.
func TestTokenize(t *testing.T) {
	text := "Кр-нин с. Покровскаго, 1834г."
	got := Tokenize(text)

	want := []Token{
		{Raw: "Кр-нин", Form: "кр-нин"},
		{Raw: "с", Form: "с", Dotted: true},
		{Raw: "Покровскаго", Form: "покровскаго"},
		{Raw: "1834", Form: "1834", Number: true},
		{Raw: "г", Form: "г", Dotted: true},
	}
	if len(got) != len(want) {
		t.Fatalf("Tokenize(%q) = %+v, want %d токенов", text, got, len(want))
	}

	for i, w := range want {
		g := got[i]
		if g.Raw != w.Raw || g.Form != w.Form || g.Dotted != w.Dotted || g.Number != w.Number {
			t.Errorf("токен %d = %+v, want %+v", i, g, w)
		}

		if text[g.Start:g.End] != g.Raw {
			t.Errorf("токен %d: text[%d:%d] = %q, want %q", i, g.Start, g.End, text[g.Start:g.End], g.Raw)
		}
	}

	if got[2].Start != strings.Index(text, "Покровскаго") {
		t.Errorf("Start «Покровскаго» = %d", got[2].Start)
	}
}

// TestTokenizeEdges: латиница не токенизируется (кроме i в кириллическом
// слове), отдельно стоящий дефис — разделитель, титло внутри слова.
func TestTokenizeEdges(t *testing.T) {
	cases := []struct {
		text  string
		forms []string
	}{
		{"Mississippi и", []string{"и"}},
		{"кот - пёс", []string{"кот", "пес"}},
		{"-кот-", []string{"кот"}},
		{"Бг҃ъ", []string{"бг"}},
		{"Iоаннъ Ѳеодоровъ", []string{"иоанн", "феодоров"}},
		{"Санктъ-Петербургъ", []string{"санкт-петербург"}},
		{"", nil},
	}
	for _, c := range cases {
		var forms []string
		for _, tk := range Tokenize(c.text) {
			forms = append(forms, tk.Form)
		}

		if strings.Join(forms, "|") != strings.Join(c.forms, "|") {
			t.Errorf("Tokenize(%q) формы = %v, want %v", c.text, forms, c.forms)
		}
	}
}
```

`internal/textnorm/reform_test.go`:

```go
package textnorm

import (
	"slices"
	"testing"
)

// TestReformVariants: дореформенные окончания прилагательных дают
// современные варианты; прочие слова — без вариантов.
func TestReformVariants(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"покровскаго", []string{"покровского"}},
		{"синяго", []string{"синего"}},
		{"новыя", []string{"новые", "новой"}},
		{"покровския", []string{"покровские", "покровской"}},
		{"кот", nil},
	}
	for _, c := range cases {
		if got := ReformVariants(c.in); !slices.Equal(got, c.want) {
			t.Errorf("ReformVariants(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Убедиться, что тесты падают**

Run: `go test ./internal/textnorm/ -count=1`
Expected: FAIL — `undefined: Tokenize`, `undefined: ReformVariants`.

- [ ] **Step 3: Реализация**

`internal/textnorm/token.go`:

```go
package textnorm

// Token — слово или число исходного текста.
type Token struct {
	Raw    string // как в тексте
	Form   string // NormalizeWord(Raw); у числа — сами цифры
	Start  int    // байтовое смещение Raw в тексте
	End    int    // байтовое смещение конца Raw
	Dotted bool   // сразу за словом точка — кандидат в сокращения
	Number bool   // токен из цифр
}
```

`internal/textnorm/tokenize.go`:

```go
package textnorm

import "unicode"

// runePos — руна текста и её байтовое смещение.
type runePos struct {
	off int
	r   rune
}

// Tokenize разбивает текст на слова и числа. Слово — кириллица (и знаки над
// ней), дефис внутри слова, латинская i рядом с кириллицей; число — ASCII-
// цифры. Остальное — разделители. Токены с пустой формой пропускаются.
func Tokenize(text string) []Token {
	p := make([]runePos, 0, len(text))
	for off, r := range text {
		p = append(p, runePos{off, r})
	}

	n := len(p)
	end := func(j int) int {
		if j < n {
			return p[j].off
		}

		return len(text)
	}

	var out []Token

	for i := 0; i < n; {
		switch {
		case isDigit(p[i].r):
			j := i
			for j < n && isDigit(p[j].r) {
				j++
			}

			raw := text[p[i].off:end(j)]
			out = append(out, Token{Raw: raw, Form: raw, Start: p[i].off, End: end(j), Number: true})
			i = j
		case isWordRune(p, i):
			j := i
			for j < n && (isWordRune(p, j) || (isHyphen(p[j].r) && isWordRune(p, j+1))) {
				j++
			}

			raw := text[p[i].off:end(j)]
			if form := NormalizeWord(raw); form != "" {
				out = append(out, Token{
					Raw: raw, Form: form, Start: p[i].off, End: end(j),
					Dotted: j < n && p[j].r == '.',
				})
			}

			i = j
		default:
			i++
		}
	}

	return out
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func isHyphen(r rune) bool { return r == '-' || r == '‐' }

func isCyrillic(p []runePos, j int) bool {
	return j >= 0 && j < len(p) && unicode.Is(unicode.Cyrillic, p[j].r)
}

// isWordRune: кириллица, комбинирующий знак или латинская i/I по соседству с
// кириллицей («Iоаннъ»).
func isWordRune(p []runePos, j int) bool {
	if j < 0 || j >= len(p) {
		return false
	}

	r := p[j].r

	switch {
	case unicode.Is(unicode.Cyrillic, r), unicode.IsMark(r):
		return true
	case r == 'i' || r == 'I':
		return isCyrillic(p, j-1) || isCyrillic(p, j+1)
	}

	return false
}
```

`internal/textnorm/reform.go`:

```go
package textnorm

import "strings"

// reformEndings — дореформенные окончания прилагательных и их современные
// варианты: -аго/-яго (род. п. м./ср. р.), -ыя/-ия (род. п. ж. р. или им. п.
// мн. ч.). Варианты пробуются при разборе, только если у формы нет точного
// словарного разбора; форма в индексе остаётся как в тексте.
var reformEndings = []struct {
	old  string
	news []string
}{
	{"аго", []string{"ого"}},
	{"яго", []string{"его"}},
	{"ыя", []string{"ые", "ой"}},
	{"ия", []string{"ие", "ей"}},
}

// ReformVariants возвращает современные написания формы с дореформенным
// окончанием; nil — если окончание не дореформенное.
func ReformVariants(form string) []string {
	for _, e := range reformEndings {
		if !strings.HasSuffix(form, e.old) {
			continue
		}

		stem := strings.TrimSuffix(form, e.old)
		out := make([]string, 0, len(e.news))

		for _, nw := range e.news {
			out = append(out, stem+nw)
		}

		return out
	}

	return nil
}
```

- [ ] **Step 4: Тесты проходят**

Run: `gofmt -l internal/textnorm && go vet ./internal/textnorm/ && go test ./internal/textnorm/ -count=1`
Expected: gofmt пусто, `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/textnorm/
git commit -m "feat(textnorm): токенизатор со смещениями и варианты дореформенных окончаний"
```

---

### Task 3: `dicts` — загрузка словарей, снимок, разбор, версия

**Files:**
- Modify: `go.mod`, `go.sum` (Go 1.27.1, gomorphy)
- Create: `internal/dicts/kind.go`, `internal/dicts/reading.go`, `internal/dicts/entry.go`, `internal/dicts/options.go`, `internal/dicts/loaded.go`, `internal/dicts/set.go`, `internal/dicts/registry.go`, `internal/dicts/deps.go`, `internal/dicts/abbrev.tsv`
- Test: `internal/dicts/helpers_test.go`, `internal/dicts/registry_test.go`, `internal/dicts/kind_test.go`

**Interfaces:**
- Consumes: gomorphy `morphology.OpenBytes`, `morphology.ImportTSV`, `morphology.NewBuilder`, `Reading.Predicted` (задача 0).
- Produces:
  - `dicts.Kind` (string) и константы `KindBase, KindAbbrev, KindSurname, KindGiven, KindPatronymic, KindToponym, KindCustom`;
  - `dicts.Reading{Normal, Tag string; Kind Kind; Predicted bool}`;
  - `dicts.Entry{Name string; Kind Kind; Origin, Hash string; Enabled bool; Info, Error string}`;
  - `dicts.Options{Dir string; Base []byte; State StateStore}`;
  - `dicts.StateStore` — `ListStates(ctx) (map[string]bool, error)`, `SaveState(ctx, name string, enabled bool) error`;
  - `dicts.Open(ctx, Options) (*Registry, error)`; методы `List() []Entry`, `Version() string`, `Parse(word string, kinds []Kind) []Reading`, `Summary() string`, `Close() error`;
  - константы имён `BaseName = "base.opencorpora"`, `AbbrevName = "abbrev.builtin"`.

- [ ] **Step 1: Go и зависимость**

```bash
go mod edit -go=1.27.1
go get github.com/amarin/gomorphy@v1.2.0   # фактическая версия из задачи 0
go build ./... && go test ./... -count=1
```
Expected: сборка и тесты зелёные на новой версии Go (gomorphy пока не импортирован — `go mod tidy` не запускать до конца задачи).

- [ ] **Step 2: Тесты (падают — пакета нет)**

`internal/dicts/helpers_test.go`:

```go
package dicts

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/amarin/gomorphy/pkg/morphology"
)

// form — одна словоформа тестового словаря: (форма, лемма, тег).
type form [3]string

// datBytes собирает словарь gomorphy из форм и возвращает его в формате GMOR.
func datBytes(t *testing.T, forms []form) []byte {
	t.Helper()

	b := morphology.NewBuilder(morphology.BuilderOptions{Language: "ru"})
	for _, f := range forms {
		if err := b.AddForm(f[0], f[1], f[2]); err != nil {
			t.Fatalf("AddForm: %v", err)
		}
	}

	d, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	path := filepath.Join(t.TempDir(), "d.dat")
	if err := d.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

// memState — StateStore в памяти.
type memState map[string]bool

func (m memState) ListStates(context.Context) (map[string]bool, error) {
	out := map[string]bool{}
	for k, v := range m {
		out[k] = v
	}

	return out, nil
}

func (m memState) SaveState(_ context.Context, name string, enabled bool) error {
	m[name] = enabled

	return nil
}

var baseForms = []form{
	{"кот", "кот", "NOUN,anim,masc sing,nomn"},
	{"кота", "кот", "NOUN,anim,masc sing,gent"},
}

var surnameForms = []form{
	{"кузнецов", "кузнецов", "NOUN,anim,masc,Surn sing,nomn"},
}

// exact — только словарные (не предсказанные) разборы.
func exact(rs []Reading) []Reading {
	var out []Reading
	for _, r := range rs {
		if !r.Predicted {
			out = append(out, r)
		}
	}

	return out
}

// openTest открывает реестр: базовый из baseForms, папка dir, состояние st.
func openTest(t *testing.T, dir string, st StateStore) *Registry {
	t.Helper()

	r, err := Open(context.Background(), Options{Dir: dir, Base: datBytes(t, baseForms), State: st})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	return r
}
```

`internal/dicts/kind_test.go`:

```go
package dicts

import "testing"

// TestKindOf: вид словаря — приставка имени до первой точки.
func TestKindOf(t *testing.T) {
	cases := map[string]Kind{
		"base.opencorpora":  KindBase,
		"abbrev.scribes":    KindAbbrev,
		"surname.volost":    KindSurname,
		"given.church":      KindGiven,
		"patronymic.x":      KindPatronymic,
		"toponym.tula":      KindToponym,
		"mine":              KindCustom,
		"something.else":    KindCustom,
	}
	for name, want := range cases {
		if got := kindOf(name); got != want {
			t.Errorf("kindOf(%q) = %q, want %q", name, got, want)
		}
	}
}
```

`internal/dicts/registry_test.go`:

```go
package dicts

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestOpenLoadsBuiltinsAndFiles: встроенные базовый и сокращения, затем файлы
// папки по алфавиту; все включены; вид — по имени файла.
func TestOpenLoadsBuiltinsAndFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "surname.test.dat"), datBytes(t, surnameForms), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "custom.x.tsv"), []byte("село\tсельцо\tNOUN\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := openTest(t, dir, memState{})

	var names []string
	for _, e := range r.List() {
		names = append(names, e.Name+":"+string(e.Kind))

		if !e.Enabled || e.Error != "" || e.Hash == "" {
			t.Errorf("словарь %s: Enabled=%v Error=%q Hash=%q", e.Name, e.Enabled, e.Error, e.Hash)
		}
	}

	want := "base.opencorpora:base|abbrev.builtin:abbrev|custom.x:custom|surname.test:surname"
	if got := strings.Join(names, "|"); got != want {
		t.Fatalf("List = %s, want %s", got, want)
	}
}

// TestParseByKinds: nil — все словари; список видов сужает разбор.
func TestParseByKinds(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "surname.test.dat"), datBytes(t, surnameForms), 0o644); err != nil {
		t.Fatal(err)
	}

	r := openTest(t, dir, memState{})

	got := exact(r.Parse("кота", nil))
	if len(got) != 1 || got[0].Normal != "кот" || got[0].Kind != KindBase {
		t.Fatalf("Parse(кота, nil) = %+v", got)
	}

	for _, rd := range r.Parse("кота", []Kind{KindSurname}) {
		if rd.Kind != KindSurname {
			t.Fatalf("Parse(кота, surname) вернул разбор вида %q", rd.Kind)
		}
	}

	got = exact(r.Parse("кузнецов", []Kind{KindSurname}))
	if len(got) != 1 || got[0].Kind != KindSurname {
		t.Fatalf("Parse(кузнецов, surname) = %+v", got)
	}
}

// TestBuiltinAbbrev: стартовый словарь сокращений встроен и многозначен.
func TestBuiltinAbbrev(t *testing.T) {
	r := openTest(t, "", memState{})

	var normals []string
	for _, rd := range exact(r.Parse("с.", []Kind{KindAbbrev})) {
		normals = append(normals, rd.Normal)
	}

	slices.Sort(normals)
	if strings.Join(normals, "|") != "село|сын" {
		t.Fatalf("Parse(с.) = %v, want [село сын]", normals)
	}
}

// TestFileReplacesBuiltinBase: base.opencorpora.dat в папке заменяет встроенный.
func TestFileReplacesBuiltinBase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "base.opencorpora.dat")
	if err := os.WriteFile(path, datBytes(t, surnameForms), 0o644); err != nil {
		t.Fatal(err)
	}

	r := openTest(t, dir, memState{})

	var bases []Entry
	for _, e := range r.List() {
		if e.Kind == KindBase {
			bases = append(bases, e)
		}
	}

	if len(bases) != 1 || bases[0].Origin != path {
		t.Fatalf("базовые словари = %+v, want один из %s", bases, path)
	}

	if len(exact(r.Parse("кота", nil))) != 0 {
		t.Fatal("встроенный базовый не заменён: «кота» разбирается")
	}
}

// TestBrokenFileListedNotUsed: битый файл — в списке с ошибкой, запуск не падает.
func TestBrokenFileListedNotUsed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "custom.bad.dat"), []byte("мусор"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := openTest(t, dir, memState{})

	for _, e := range r.List() {
		if e.Name == "custom.bad" && e.Error == "" {
			t.Fatal("custom.bad без ошибки")
		}
	}

	if got := exact(r.Parse("кота", nil)); len(got) != 1 {
		t.Fatalf("Parse(кота) = %+v", got)
	}
}

// TestNoBase: без встроенного базового — только сокращения.
func TestNoBase(t *testing.T) {
	r, err := Open(t.Context(), Options{State: memState{}})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	for _, e := range r.List() {
		if e.Kind == KindBase {
			t.Fatalf("неожиданный базовый словарь %+v", e)
		}
	}

	if !strings.Contains(r.Summary(), "базовый: нет") {
		t.Fatalf("Summary = %q", r.Summary())
	}
}

// TestVersionStable: одинаковое содержимое — одинаковая версия; другое — другая.
func TestVersionStable(t *testing.T) {
	a := openTest(t, "", memState{})
	b := openTest(t, "", memState{})

	if a.Version() == "" || a.Version() != b.Version() {
		t.Fatalf("версии %q и %q", a.Version(), b.Version())
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "surname.test.dat"), datBytes(t, surnameForms), 0o644); err != nil {
		t.Fatal(err)
	}

	if c := openTest(t, dir, memState{}); c.Version() == a.Version() {
		t.Fatal("добавленный словарь не изменил версию")
	}
}
```

- [ ] **Step 3: Убедиться, что тесты падают**

Run: `go test ./internal/dicts/ -count=1`
Expected: FAIL — `undefined: Open` и т.п.

- [ ] **Step 4: Реализация**

`internal/dicts/kind.go`:

```go
// Package dicts — реестр словарей морфологии gomorphy: встроенные базовый и
// сокращений, файлы папки словарей, состояние «включён», неизменяемый снимок
// включённых для разбора и его версия (docs/search/normalization.md).
package dicts

import "strings"

// Kind — вид словаря; задаёт, какие поля им разбираются (профили normalizer).
type Kind string

const (
	KindBase       Kind = "base"
	KindAbbrev     Kind = "abbrev"
	KindSurname    Kind = "surname"
	KindGiven      Kind = "given"
	KindPatronymic Kind = "patronymic"
	KindToponym    Kind = "toponym"
	KindCustom     Kind = "custom"
)

var knownKinds = map[string]Kind{
	"base": KindBase, "abbrev": KindAbbrev, "surname": KindSurname, "given": KindGiven,
	"patronymic": KindPatronymic, "toponym": KindToponym,
}

// kindOf — вид по имени словаря «<kind>.<name>»; иначе KindCustom.
func kindOf(name string) Kind {
	prefix, _, ok := strings.Cut(name, ".")
	if !ok {
		return KindCustom
	}

	if k, known := knownKinds[prefix]; known {
		return k
	}

	return KindCustom
}
```

`internal/dicts/reading.go`:

```go
package dicts

// Reading — разбор слова одним из словарей.
type Reading struct {
	Normal    string // лемма как в словаре (может содержать ё)
	Tag       string // теги gomorphy, первый — часть речи
	Kind      Kind   // вид словаря, давшего разбор
	Predicted bool   // предсказан по окончанию, слова в словаре нет
}
```

`internal/dicts/entry.go`:

```go
package dicts

// Entry — словарь в списке реестра (CLI, позже — UI P7).
type Entry struct {
	Name    string // имя файла без расширения; встроенные — BaseName, AbbrevName
	Kind    Kind
	Origin  string // "встроенный" или путь к файлу
	Hash    string // sha256 содержимого, hex
	Enabled bool   // состояние пользователя; битый словарь не участвует и включённым
	Info    string // источник и версия из BuildInfo — атрибуция (CC BY-SA у OpenCorpora)
	Error   string // ошибка загрузки; пусто — загружен
}
```

`internal/dicts/options.go`:

```go
package dicts

// Options — параметры открытия реестра.
type Options struct {
	Dir   string     // папка словарей; "" — без папки
	Base  []byte     // встроенный базовый словарь (GMOR); nil — не встроен
	State StateStore // состояние «включён»
}
```

`internal/dicts/deps.go`:

```go
package dicts

import "context"

// StateStore — хранилище состояния словарей: включён ли словарь по имени.
// Словарь без записи считается включённым.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package dicts
type StateStore interface {
	ListStates(ctx context.Context) (map[string]bool, error)
	SaveState(ctx context.Context, name string, enabled bool) error
}
```

`internal/dicts/loaded.go`:

```go
package dicts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/amarin/gomorphy/pkg/morphology"
)

// originBuiltin — Origin встроенных словарей.
const originBuiltin = "встроенный"

// loaded — словарь реестра: описание и открытый словарь (nil — не загрузился).
// data держит буфер, поверх которого открыт словарь (OpenBytes не копирует).
type loaded struct {
	entry Entry
	d     *morphology.Dictionary
	data  []byte
}

// openFunc открывает словарь из буфера.
type openFunc func([]byte) (*morphology.Dictionary, error)

// openTSV — словарь из TSV «lemma<TAB>wordform[<TAB>tags]».
func openTSV(data []byte) (*morphology.Dictionary, error) {
	return morphology.ImportTSV(bytes.NewReader(data), morphology.BuilderOptions{Language: "ru", Source: "tsv"})
}

// loadBytes открывает словарь из буфера; ошибка — в Entry.Error.
func loadBytes(name, origin string, data []byte, open openFunc) *loaded {
	sum := sha256.Sum256(data)
	l := &loaded{
		entry: Entry{Name: name, Kind: kindOf(name), Origin: origin, Hash: hex.EncodeToString(sum[:]), Enabled: true},
		data:  data,
	}

	d, err := open(data)
	if err != nil {
		l.entry.Error = err.Error()

		return l
	}

	l.d = d
	l.entry.Info = infoOf(d)

	return l
}

// loadDir загружает *.dat и *.tsv папки (создаёт её, если нет), по алфавиту.
func loadDir(dir string) ([]*loaded, error) {
	if dir == "" {
		return nil, nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var out []*loaded

	for _, it := range items {
		ext := filepath.Ext(it.Name())
		if !it.Type().IsRegular() || (ext != ".dat" && ext != ".tsv") {
			continue
		}

		path := filepath.Join(dir, it.Name())
		name := strings.TrimSuffix(it.Name(), ext)

		data, err := os.ReadFile(path)
		if err != nil {
			out = append(out, &loaded{entry: Entry{Name: name, Kind: kindOf(name), Origin: path, Error: err.Error(), Enabled: true}})

			continue
		}

		open := openFunc(morphology.OpenBytes)
		if ext == ".tsv" {
			open = openTSV
		}

		out = append(out, loadBytes(name, path, data, open))
	}

	sort.Slice(out, func(i, j int) bool { return out[i].entry.Name < out[j].entry.Name })

	return out, nil
}

// infoOf — источник, версия и адрес словаря для атрибуции.
func infoOf(d *morphology.Dictionary) string {
	bi := d.Info()
	if bi == nil {
		return ""
	}

	var parts []string

	for _, s := range []string{bi.Source, bi.SourceVersion, bi.SourceURL} {
		if s != "" {
			parts = append(parts, s)
		}
	}

	return strings.Join(parts, " ")
}
```

`internal/dicts/set.go`:

```go
package dicts

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// set — неизменяемый снимок включённых загруженных словарей и его версия.
type set struct {
	dicts   []*loaded
	version string
}

// newSet собирает снимок; версия — sha256 по (имя, вид, хэш) включённых в порядке.
func newSet(all []*loaded) *set {
	s := &set{}
	h := sha256.New()

	for _, l := range all {
		if !l.entry.Enabled || l.d == nil {
			continue
		}

		s.dicts = append(s.dicts, l)
		fmt.Fprintf(h, "%s\t%s\t%s\n", l.entry.Name, l.entry.Kind, l.entry.Hash)
	}

	s.version = hex.EncodeToString(h.Sum(nil))

	return s
}
```

`internal/dicts/registry.go`:

```go
package dicts

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/amarin/gomorphy/pkg/morphology"
)

// Имена встроенных словарей.
const (
	BaseName   = "base.opencorpora"
	AbbrevName = "abbrev.builtin"
)

// ErrUnknownDictionary — словаря с таким именем нет.
var ErrUnknownDictionary = errors.New("словарь не найден")

//go:embed abbrev.tsv
var builtinAbbrev []byte

// Registry — все словари (встроенные и из папки), их состояние и текущий
// снимок включённых. Разбор читает снимок без блокировки; смена состояния
// собирает новый снимок и подменяет его атомарно.
type Registry struct {
	mu      sync.Mutex
	all     []*loaded
	state   StateStore
	current atomic.Pointer[set]
}

// Open загружает встроенные словари и папку, применяет сохранённое состояние.
// Битые файлы не прерывают открытие (Entry.Error).
func Open(ctx context.Context, opts Options) (*Registry, error) {
	var all []*loaded
	if opts.Base != nil {
		all = append(all, loadBytes(BaseName, originBuiltin, opts.Base, morphology.OpenBytes))
	}

	all = append(all, loadBytes(AbbrevName, originBuiltin, builtinAbbrev, openTSV))

	files, err := loadDir(opts.Dir)
	if err != nil {
		_ = closeAll(all)

		return nil, fmt.Errorf("папка словарей: %w", err)
	}

	all = mergeFiles(all, files)

	states, err := opts.State.ListStates(ctx)
	if err != nil {
		_ = closeAll(all)

		return nil, fmt.Errorf("состояние словарей: %w", err)
	}

	for _, l := range all {
		if on, ok := states[l.entry.Name]; ok {
			l.entry.Enabled = on
		}
	}

	r := &Registry{all: all, state: opts.State}
	r.current.Store(newSet(all))

	return r, nil
}

// mergeFiles: файл с именем встроенного заменяет его на месте, прочие —
// в конец (уже по алфавиту).
func mergeFiles(builtin, files []*loaded) []*loaded {
	for _, f := range files {
		i := slices.IndexFunc(builtin, func(b *loaded) bool { return b.entry.Name == f.entry.Name })
		if i >= 0 {
			_ = closeAll(builtin[i : i+1])
			builtin[i] = f

			continue
		}

		builtin = append(builtin, f)
	}

	return builtin
}

// List — копии описаний всех словарей в порядке разбора.
func (r *Registry) List() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]Entry, 0, len(r.all))
	for _, l := range r.all {
		out = append(out, l.entry)
	}

	return out
}

// Version — версия набора включённых словарей (P2 сверяет по ней индекс).
func (r *Registry) Version() string { return r.current.Load().version }

// Parse разбирает слово включёнными словарями указанных видов (nil — всеми)
// в порядке реестра.
func (r *Registry) Parse(word string, kinds []Kind) []Reading {
	var out []Reading

	for _, l := range r.current.Load().dicts {
		if kinds != nil && !slices.Contains(kinds, l.entry.Kind) {
			continue
		}

		for _, rd := range l.d.Parse(word) {
			out = append(out, Reading{Normal: rd.Normal, Tag: rd.Tag, Kind: l.entry.Kind, Predicted: rd.Predicted})
		}
	}

	return out
}

// Summary — строка для журнала запуска.
func (r *Registry) Summary() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	on, base := 0, "нет"

	for _, l := range r.all {
		if l.entry.Enabled && l.d != nil {
			on++

			if l.entry.Kind == KindBase {
				base = "есть"
			}
		}
	}

	return fmt.Sprintf("словари: %d из %d включено; базовый: %s", on, len(r.all), base)
}

// Close освобождает словари; после Close реестром пользоваться нельзя.
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return closeAll(r.all)
}

func closeAll(ls []*loaded) error {
	var errs []error

	for _, l := range ls {
		if l.d != nil {
			errs = append(errs, l.d.Close())
		}
	}

	return errors.Join(errs...)
}
```

`internal/dicts/abbrev.tsv` — стартовый словарь сокращений (`lemma<TAB>wordform<TAB>tags`):

```
# Стартовый словарь сокращений писцов (docs/search/normalization.md «Сокращения»).
# Формат: лемма<TAB>сокращение<TAB>теги. Точка — часть сокращения.
село	с.	NOUN
сын	с.	NOUN
деревня	д.	NOUN
дочь	д.	NOUN
деревня	дер.	NOUN
деревня	дер	NOUN
сельцо	с-цо	NOUN
погост	пог.	NOUN
слобода	сл.	NOUN
поселок	пос.	NOUN
город	г.	NOUN
год	г.	NOUN
церковь	ц.	NOUN
губерния	губ.	NOUN
губерния	губ	NOUN
уезд	у.	NOUN
уезд	уез.	NOUN
волость	вол.	NOUN
волость	вол-ть	NOUN
приход	прих.	NOUN
крестьянин	кр-нин	NOUN
крестьянин	крест.	NOUN
крестьянка	кр-ка	NOUN
мещанин	мещ.	NOUN
однодворец	однодв.	NOUN
отставной	отст.	ADJF
рядовой	рядов.	NOUN
солдат	солд.	NOUN
священник	свящ.	NOUN
диакон	диак.	NOUN
пономарь	пон.	NOUN
вдова	вд.	NOUN
```

(разделитель — табуляция; редактор не должен заменить её пробелами.)

- [ ] **Step 5: Моки и тесты**

Run: `go generate ./internal/dicts/ && go mod tidy && gofmt -l internal && go vet ./internal/dicts/ && go test ./internal/dicts/ -count=1`
Expected: создан `internal/dicts/deps_test.go` (MockStateStore), gofmt пусто, `ok`. (Пока в пакете нет директивы генерации базового словаря — она появится в задаче 5.)

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/dicts/
git commit -m "feat(dicts): реестр словарей gomorphy — встроенные, папка, снимок, версия; Go 1.27.1"
```

---

### Task 4: `dicts` — состояние в SQLite и включение/выключение

**Files:**
- Create: `internal/storage/schema_dicts.go`, `internal/dicts/sqlstore_state.go`
- Modify: `internal/storage/db.go` (DDL), `internal/store/sqlstore/fkgraph_test.go` (`serviceTables`), `internal/dicts/registry.go` (метод `SetEnabled`)
- Test: `internal/storage/schema_dicts_test.go`, `internal/dicts/sqlstore_state_test.go`, `internal/dicts/registry_state_test.go`

**Interfaces:**
- Consumes: `storage.Open`, `storage.DB` (`ExecContext`, `QueryContext`); `Registry`, `StateStore`, `MockStateStore` (задача 3).
- Produces: `storage` — таблица `dictionaries(name TEXT PRIMARY KEY, enabled INTEGER NOT NULL)`; `dicts.SQLStateStore`, `dicts.NewSQLStateStore(db *storage.DB) *SQLStateStore`; `(*Registry).SetEnabled(ctx, name string, on bool) error` (нет имени — `ErrUnknownDictionary`).

- [ ] **Step 1: Тесты**

`internal/storage/schema_dicts_test.go`:

```go
package storage

import (
	"fmt"
	"testing"
)

// TestDictionariesTable: служебная таблица состояния словарей создаётся схемой.
func TestDictionariesTable(t *testing.T) {
	db := openTestDB(t)

	rows, err := db.Query(`SELECT name FROM pragma_table_info('dictionaries') ORDER BY cid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var cols []string

	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}

		cols = append(cols, c)
	}

	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	if fmt.Sprint(cols) != "[name enabled]" {
		t.Fatalf("колонки dictionaries %v, want [name enabled]", cols)
	}
}
```

`internal/dicts/sqlstore_state_test.go`:

```go
package dicts

import (
	"testing"

	"github.com/amarin/genodex/internal/storage"
)

// newStateStore — SQLStateStore на временном каталоге данных.
func newStateStore(t *testing.T) *SQLStateStore {
	t.Helper()

	st, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	return NewSQLStateStore(st.DB())
}

// TestSQLStateStore: пустое состояние, запись, перезапись.
func TestSQLStateStore(t *testing.T) {
	s := newStateStore(t)
	ctx := t.Context()

	got, err := s.ListStates(ctx)
	if err != nil || len(got) != 0 {
		t.Fatalf("ListStates = %v, %v; want пусто", got, err)
	}

	if err := s.SaveState(ctx, "surname.x", false); err != nil {
		t.Fatal(err)
	}

	if err := s.SaveState(ctx, "surname.x", true); err != nil {
		t.Fatal(err)
	}

	if err := s.SaveState(ctx, "abbrev.builtin", false); err != nil {
		t.Fatal(err)
	}

	got, err = s.ListStates(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 || !got["surname.x"] || got["abbrev.builtin"] {
		t.Fatalf("ListStates = %v", got)
	}
}
```

`internal/dicts/registry_state_test.go`:

```go
package dicts

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/mock/gomock"
)

// TestSetEnabled: выключение сохраняется, убирает словарь из разбора, меняет
// версию и переживает повторное открытие.
func TestSetEnabled(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "surname.test.dat"), datBytes(t, surnameForms), 0o644); err != nil {
		t.Fatal(err)
	}

	st := newStateStore(t)
	r := openTest(t, dir, st)
	v1 := r.Version()

	if err := r.SetEnabled(t.Context(), "surname.test", false); err != nil {
		t.Fatal(err)
	}

	if r.Version() == v1 {
		t.Fatal("версия не изменилась")
	}

	if got := r.Parse("кузнецов", []Kind{KindSurname}); len(got) != 0 {
		t.Fatalf("выключенный словарь разбирает: %+v", got)
	}

	again := openTest(t, dir, st)
	for _, e := range again.List() {
		if e.Name == "surname.test" && e.Enabled {
			t.Fatal("состояние не пережило повторное открытие")
		}
	}
}

// TestSetEnabledUnknown: неизвестное имя — ErrUnknownDictionary, состояние не пишется.
func TestSetEnabledUnknown(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := NewMockStateStore(ctrl)
	st.EXPECT().ListStates(gomock.Any()).Return(map[string]bool{}, nil)

	r := openTest(t, "", st)

	if err := r.SetEnabled(t.Context(), "нет-такого", false); !errors.Is(err, ErrUnknownDictionary) {
		t.Fatalf("err = %v, want ErrUnknownDictionary", err)
	}
}

// TestOpenStateError: ошибка чтения состояния прерывает открытие.
func TestOpenStateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := NewMockStateStore(ctrl)
	st.EXPECT().ListStates(gomock.Any()).Return(nil, errors.New("сбой"))

	if _, err := Open(t.Context(), Options{State: st}); err == nil {
		t.Fatal("Open без ошибки при сбое состояния")
	}
}
```

- [ ] **Step 2: Убедиться, что тесты падают**

Run: `go test ./internal/storage/ ./internal/dicts/ -count=1`
Expected: FAIL — нет таблицы `dictionaries`, `undefined: NewSQLStateStore`, `r.SetEnabled undefined`.

- [ ] **Step 3: Реализация**

`internal/storage/schema_dicts.go`:

```go
package storage

// dictsSchemaDDL — состояние словарей морфологии (internal/dicts): включён ли
// словарь по имени. Служебная таблица вне generic-системы сущностей
// (docs/search/normalization.md «Папка словарей»); словарь без строки включён.
var dictsSchemaDDL = []string{
	`CREATE TABLE IF NOT EXISTS dictionaries (
		name    TEXT PRIMARY KEY,
		enabled INTEGER NOT NULL
	)`,
}
```

`internal/storage/db.go`, в `OpenDB` после `ddl = append(ddl, authSchemaDDL...)`:

```go
	ddl = append(ddl, dictsSchemaDDL...)
```

`internal/store/sqlstore/fkgraph_test.go`, в `serviceTables` добавить `"dictionaries": true` (в строку с таблицами auth):

```go
	"owners": true, "sessions": true, "api_tokens": true, "invites": true,
	"dictionaries": true,
```

`internal/dicts/sqlstore_state.go`:

```go
package dicts

import (
	"context"

	"github.com/amarin/genodex/internal/storage"
)

// SQLStateStore — StateStore поверх таблицы dictionaries общей SQLite.
type SQLStateStore struct {
	db *storage.DB
}

// NewSQLStateStore оборачивает открытое соединение хранилища.
func NewSQLStateStore(db *storage.DB) *SQLStateStore {
	return &SQLStateStore{db: db}
}

var _ StateStore = (*SQLStateStore)(nil)

// ListStates читает сохранённые состояния словарей.
func (s *SQLStateStore) ListStates(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, enabled FROM dictionaries`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]bool{}

	for rows.Next() {
		var (
			name string
			on   int
		)

		if err := rows.Scan(&name, &on); err != nil {
			return nil, err
		}

		out[name] = on != 0
	}

	return out, rows.Err()
}

// SaveState записывает или перезаписывает состояние словаря.
func (s *SQLStateStore) SaveState(ctx context.Context, name string, enabled bool) error {
	on := 0
	if enabled {
		on = 1
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO dictionaries(name, enabled) VALUES (?, ?)
		 ON CONFLICT(name) DO UPDATE SET enabled = excluded.enabled`, name, on)

	return err
}
```

`internal/dicts/registry.go` — добавить метод (после `List`):

```go
// SetEnabled включает или выключает словарь: сохраняет состояние и
// подменяет снимок. Нет словаря — ErrUnknownDictionary.
func (r *Registry) SetEnabled(ctx context.Context, name string, on bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, l := range r.all {
		if l.entry.Name != name {
			continue
		}

		if err := r.state.SaveState(ctx, name, on); err != nil {
			return fmt.Errorf("состояние словаря %s: %w", name, err)
		}

		l.entry.Enabled = on
		r.current.Store(newSet(r.all))

		return nil
	}

	return fmt.Errorf("%w: %s", ErrUnknownDictionary, name)
}
```

- [ ] **Step 4: Тесты проходят**

Run: `gofmt -l internal && go vet ./internal/... && go test ./internal/storage/ ./internal/store/sqlstore/ ./internal/dicts/ -count=1`
Expected: gofmt пусто, `ok` (в т.ч. `TestEntityTablesMatchSchema` с новой служебной таблицей).

- [ ] **Step 5: Commit**

```bash
git add internal/storage/schema_dicts.go internal/storage/schema_dicts_test.go internal/storage/db.go \
  internal/store/sqlstore/fkgraph_test.go internal/dicts/
git commit -m "feat(dicts): состояние словарей в таблице dictionaries, включение и выключение на ходу"
```

---

### Task 5: встроенный базовый словарь — загрузка, `go generate`, тег `nobasedict`

**Files:**
- Create: `internal/dicts/basefetch/fetch.go`, `internal/dicts/basegen/main.go`, `internal/dicts/embedded_base.go`, `internal/dicts/embedded_base_none.go`, `internal/dicts/base/.gitignore`
- Test: `internal/dicts/embedded_base_test.go`

**Interfaces:**
- Consumes: gomorphy `pymorphy.NewLoader`, `Loader.Sync`, `Loader.UnpackedDirPath`, `Loader.LocalVersion`, `morphology.OpenPyMorphyDense`, `Dictionary.SaveTo`.
- Produces: `basefetch.Fetch(dst string) (version string, err error)`; `dicts.EmbeddedBase() []byte` (nil со сборкой `nobasedict`); файл `internal/dicts/base/base.opencorpora.dat` (генерат, не в git).

`basefetch` — отдельный пакет, потому что `go generate` запускает `go run ./basegen`, а тот не должен импортировать `dicts` (пакет `dicts` не компилируется, пока нет встраиваемого файла).

- [ ] **Step 1: Тест**

`internal/dicts/embedded_base_test.go`:

```go
package dicts

import "testing"

// TestEmbeddedBaseOpens: встроенный базовый словарь (если собран без
// nobasedict) открывается и разбирает обычное слово.
func TestEmbeddedBaseOpens(t *testing.T) {
	data := EmbeddedBase()
	if data == nil {
		t.Skip("сборка без базового словаря (nobasedict)")
	}

	r, err := Open(t.Context(), Options{Base: data, State: memState{}})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	got := exact(r.Parse("кота", []Kind{KindBase}))
	if len(got) == 0 || got[0].Normal != "кот" {
		t.Fatalf("Parse(кота) = %+v, want лемму «кот»", got)
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./internal/dicts/ -run EmbeddedBase -count=1`
Expected: FAIL — `undefined: EmbeddedBase`.

- [ ] **Step 3: Реализация**

`internal/dicts/basefetch/fetch.go`:

```go
// Package basefetch скачивает и компилирует базовый словарь (OpenCorpora
// через pymorphy2-dicts-ru с PyPI). Нужен интернет. Используется генератором
// встроенного словаря и командой genodex dicts fetch.
package basefetch

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/amarin/gomorphy/pkg/morphology"
	"github.com/amarin/gomorphy/pkg/pymorphy"
)

// Fetch скачивает pymorphy2-dicts-ru, компилирует его и атомарно сохраняет в
// dst. Возвращает версию пакета словаря.
func Fetch(dst string) (string, error) {
	work, err := os.MkdirTemp("", "genodex-basedict-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)

	loader := pymorphy.NewLoader(work)
	if err := loader.Sync(false); err != nil {
		return "", fmt.Errorf("загрузка pymorphy2-dicts-ru: %w", err)
	}

	d, err := morphology.OpenPyMorphyDense(loader.UnpackedDirPath())
	if err != nil {
		return "", fmt.Errorf("компиляция словаря: %w", err)
	}
	defer d.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}

	tmp := dst + ".tmp"
	if err := d.SaveTo(tmp); err != nil {
		return "", fmt.Errorf("сохранение словаря: %w", err)
	}

	if err := os.Rename(tmp, dst); err != nil {
		return "", err
	}

	version, _ := loader.LocalVersion()

	return version, nil
}
```

`internal/dicts/basegen/main.go`:

```go
// Команда basegen — генератор встроенного базового словаря для go generate:
// скачивает и компилирует словарь, если файла ещё нет (или -force).
package main

import (
	"flag"
	"log"
	"os"

	"github.com/amarin/genodex/internal/dicts/basefetch"
)

func main() {
	out := flag.String("out", "base/base.opencorpora.dat", "куда сохранить словарь")
	force := flag.Bool("force", false, "перекачать, даже если файл есть")
	flag.Parse()

	if _, err := os.Stat(*out); err == nil && !*force {
		log.Printf("базовый словарь уже есть: %s (-force — перекачать)", *out)

		return
	}

	version, err := basefetch.Fetch(*out)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("базовый словарь pymorphy2-dicts-ru %s → %s", version, *out)
}
```

`internal/dicts/embedded_base.go`:

```go
//go:build !nobasedict

package dicts

import _ "embed"

// Встроенный базовый словарь собирается go generate (нужен интернет) —
// как web/dist для фронта. Сборка с тегом nobasedict обходится без него.
//
//go:generate go run ./basegen -out base/base.opencorpora.dat
//go:embed base/base.opencorpora.dat
var embeddedBase []byte

// EmbeddedBase — встроенный базовый словарь (формат GMOR).
func EmbeddedBase() []byte { return embeddedBase }
```

`internal/dicts/embedded_base_none.go`:

```go
//go:build nobasedict

package dicts

// EmbeddedBase — сборка без встроенного базового словаря: nil. Словарь можно
// положить в папку словарей позже (genodex dicts fetch).
func EmbeddedBase() []byte { return nil }
```

`internal/dicts/base/.gitignore`:

```
# Генерат go generate ./internal/dicts/ — не коммитится.
*.dat
*.tmp
```

- [ ] **Step 4: Генерация и тесты в обоих режимах**

Run:
```bash
go generate ./internal/dicts/
go build ./... && go vet ./... && go test ./internal/dicts/ -count=1
go build -tags nobasedict ./... && go vet -tags nobasedict ./... && go test -tags nobasedict ./internal/dicts/ -count=1
git status --short internal/dicts/base/
```
Expected: генератор скачал и сохранил `base.opencorpora.dat` (~13 МБ); всё зелёное; `TestEmbeddedBaseOpens` проходит в обычной сборке и пропускается с `nobasedict`; `git status` не показывает `.dat` (только `.gitignore`).

- [ ] **Step 5: Commit**

```bash
git add internal/dicts/basefetch/ internal/dicts/basegen/ internal/dicts/embedded_base.go \
  internal/dicts/embedded_base_none.go internal/dicts/embedded_base_test.go internal/dicts/base/.gitignore go.mod go.sum
git commit -m "feat(dicts): встроенный базовый словарь через go generate, сборка nobasedict без него"
```

---

### Task 6: `normalizer` — разбор текста в термины

**Files:**
- Create: `internal/normalizer/deps.go`, `internal/normalizer/profile.go`, `internal/normalizer/lemma.go`, `internal/normalizer/term.go`, `internal/normalizer/tag.go`, `internal/normalizer/normalizer.go`
- Test: `internal/normalizer/fake_test.go`, `internal/normalizer/normalizer_test.go`, `internal/normalizer/lemma_test.go`

**Interfaces:**
- Consumes: `textnorm.Tokenize`, `textnorm.Token`, `textnorm.NormalizeWord`, `textnorm.ReformVariants` (задачи 1–2); `dicts.Reading`, `dicts.Kind*` (задача 3).
- Produces:
  - `normalizer.Dictionaries` — `Parse(word string, kinds []dicts.Kind) []dicts.Reading` (реализует `*dicts.Registry`);
  - `normalizer.Profile` — `ProfileText`, `ProfileName`, `ProfilePlace`;
  - `normalizer.Flag` — `FlagAmbiguous`, `FlagPredicted`, `FlagUnknown`, `FlagAbbrev`; `(Flag).String() string`;
  - `normalizer.Lemma{Text string; Flags Flag}`, `normalizer.Term{Token textnorm.Token; Form string; Lemmas []Lemma}`;
  - `normalizer.New(d Dictionaries) *Normalizer`, `(*Normalizer).Analyze(text string, p Profile) []Term`.

- [ ] **Step 1: Тесты**

`internal/normalizer/fake_test.go`:

```go
package normalizer

import (
	"slices"

	"github.com/amarin/genodex/internal/dicts"
)

// fakeDicts — разборы по форме слова; Kind у разбора — вид словаря.
type fakeDicts map[string][]dicts.Reading

func (f fakeDicts) Parse(word string, kinds []dicts.Kind) []dicts.Reading {
	var out []dicts.Reading

	for _, r := range f[word] {
		if kinds == nil || slices.Contains(kinds, r.Kind) {
			out = append(out, r)
		}
	}

	return out
}

const (
	base   = dicts.KindBase
	abbrev = dicts.KindAbbrev
)

var fake = fakeDicts{
	"кот":         {{Normal: "кот", Tag: "NOUN,anim,masc sing,nomn", Kind: base}},
	"кота":        {{Normal: "кот", Tag: "NOUN,anim,masc sing,gent", Kind: base}},
	"пса":         {{Normal: "пёс", Tag: "NOUN,anim,masc sing,gent", Kind: base}},
	"стали":       {{Normal: "сталь", Tag: "NOUN,inan,femn sing,gent", Kind: base}, {Normal: "стать", Tag: "VERB,perf,intr plur,past,indc", Kind: base}},
	"в":           {{Normal: "в", Tag: "PREP", Kind: base}},
	"селе":        {{Normal: "село", Tag: "NOUN,inan,neut sing,loct", Kind: base}},
	"с.":          {{Normal: "село", Tag: "NOUN", Kind: abbrev}, {Normal: "сын", Tag: "NOUN", Kind: abbrev}},
	"кр-нин":      {{Normal: "крестьянин", Tag: "NOUN", Kind: abbrev}},
	"петербург":   {{Normal: "петербург", Tag: "NOUN,inan,masc,Geox sing,nomn", Kind: base}},
	"губ":         {{Normal: "губа", Tag: "NOUN,inan,femn plur,gent", Kind: base}, {Normal: "губерния", Tag: "NOUN", Kind: abbrev}},
	"дорожкиной":  {{Normal: "дорожкина", Tag: "NOUN,anim,femn sing,gent", Kind: base, Predicted: true}},
	"кузнецова":   {{Normal: "кузнецов", Tag: "NOUN,anim,masc,Surn sing,gent", Kind: base}, {Normal: "кузнецова", Tag: "ADJF", Kind: base, Predicted: true}},
	"покровского": {{Normal: "покровский", Tag: "ADJF,Geox masc,sing,gent", Kind: base}},
	"ивана":       {{Normal: "иван", Tag: "NOUN,anim,masc,Name sing,gent", Kind: base}},
	"дер":         {{Normal: "деревня", Tag: "NOUN", Kind: abbrev}, {Normal: "дерь", Tag: "NOUN", Kind: abbrev, Predicted: true}},
}
```

`internal/normalizer/normalizer_test.go`:

```go
package normalizer

import (
	"fmt"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
)

// newTest — нормализатор поверх мока, делегирующего в fake.
func newTest(t *testing.T) *Normalizer {
	t.Helper()

	m := NewMockDictionaries(gomock.NewController(t))
	m.EXPECT().Parse(gomock.Any(), gomock.Any()).DoAndReturn(fake.Parse).AnyTimes()

	return New(m)
}

// show — «форма=лемма[флаги],лемма[флаги]; …» для сравнения в тестах.
func show(terms []Term) string {
	var parts []string

	for _, t := range terms {
		var ls []string
		for _, l := range t.Lemmas {
			ls = append(ls, fmt.Sprintf("%s[%s]", l.Text, l.Flags))
		}

		parts = append(parts, t.Form+"="+strings.Join(ls, ","))
	}

	return strings.Join(parts, "; ")
}

func TestAnalyze(t *testing.T) {
	cases := []struct {
		name, text string
		p          Profile
		want       string
	}{
		{"лемма", "Кота", ProfileText, "кота=кот[]"},
		{"лемма нормализуется", "пса", ProfileText, "пса=пес[]"},
		{"омонимия", "стали", ProfileText, "стали=сталь[неоднозн],стать[неоднозн]"},
		{"стоп-слово", "кот в селе", ProfileText, "кот=кот[]; селе=село[]"},
		{"сокращение с точкой", "с. Кота", ProfileText, "с.=село[неоднозн,сокр],сын[неоднозн,сокр]; кота=кот[]"},
		{"точка — не сокращение", "кот.", ProfileText, "кот=кот[]"},
		{"сокращение с дефисом", "кр-нин", ProfileText, "кр-нин=крестьянин[сокр]"},
		{"составное слово", "Санктъ-Петербургъ", ProfileText, "санкт-петербург=санкт-петербург[неизв]; санкт=санкт[неизв]; петербург=петербург[]"},
		{"сокращение без точки и слово", "губ", ProfileText, "губ=губа[неоднозн],губерния[неоднозн,сокр]"},
		{"предсказание сокращений не берётся", "дер", ProfileText, "дер=деревня[сокр]"},
		{"предсказанная лемма", "Дорожкиной", ProfileText, "дорожкиной=дорожкина[предск]"},
		{"точный разбор важнее предсказания", "Кузнецова", ProfileText, "кузнецова=кузнецов[]"},
		{"неизвестное слово и число", "Дарожкн 1834", ProfileText, "дарожкн=дарожкн[неизв]; 1834=1834[неизв]"},
		{"дореформенное окончание", "Покровскаго", ProfileText, "покровскаго=покровский[]"},
		{"имя: обычные слова не разбираются", "стали", ProfileName, "стали=стали[неизв]"},
		{"имя: разбор с Name проходит", "Ивана", ProfileName, "ивана=иван[]"},
		{"место: сокращения разбираются", "с.", ProfilePlace, "с.=село[неоднозн,сокр],сын[неоднозн,сокр]"},
	}
	n := newTest(t)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := show(n.Analyze(c.text, c.p)); got != c.want {
				t.Errorf("Analyze(%q) = %q, want %q", c.text, got, c.want)
			}
		})
	}
}

// TestAnalyzeKeepsToken: термин несёт исходный токен (смещения — для фрагментов P3).
func TestAnalyzeKeepsToken(t *testing.T) {
	text := "в селе Кота"
	terms := newTest(t).Analyze(text, ProfileText)

	last := terms[len(terms)-1]
	if text[last.Token.Start:last.Token.End] != "Кота" {
		t.Fatalf("токен %+v", last.Token)
	}
}
```

`internal/normalizer/lemma_test.go`:

```go
package normalizer

import "testing"

func TestFlagString(t *testing.T) {
	cases := map[Flag]string{
		0:                            "",
		FlagAmbiguous:                "неоднозн",
		FlagAmbiguous | FlagAbbrev:   "неоднозн,сокр",
		FlagPredicted:                "предск",
		FlagUnknown:                  "неизв",
	}
	for f, want := range cases {
		if got := f.String(); got != want {
			t.Errorf("Flag(%d).String() = %q, want %q", f, got, want)
		}
	}
}
```

- [ ] **Step 2: Убедиться, что тесты падают**

Run: `go test ./internal/normalizer/ -count=1`
Expected: FAIL — `undefined: New`, `undefined: NewMockDictionaries` и т.п.

- [ ] **Step 3: Реализация**

`internal/normalizer/deps.go`:

```go
// Package normalizer — общий нормализатор текста для поиска, NER (E9), дублей
// (E10) и сопоставления (F5): токены textnorm, сокращения, леммы по словарям
// dicts, стоп-слова (docs/search/normalization.md). Один на всё — два
// нормализатора разойдутся.
package normalizer

import "github.com/amarin/genodex/internal/dicts"

// Dictionaries — разбор слова словарями указанных видов (nil — всеми).
// Реализует *dicts.Registry.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package normalizer
type Dictionaries interface {
	Parse(word string, kinds []dicts.Kind) []dicts.Reading
}
```

`internal/normalizer/profile.go`:

```go
package normalizer

import "github.com/amarin/genodex/internal/dicts"

// Profile — какими словарями разбирать поле: против омонимии имён и обычных
// слов («Вера», «Мороз») поля имени и места разбираются своими наборами.
type Profile int

const (
	ProfileText  Profile = iota // свободный текст — все словари
	ProfileName                 // поля имени — словари имён и Name/Surn/Patr базового
	ProfilePlace                // поля места — топонимы, сокращения и Geox базового
)

// profileSpec — виды словарей профиля (nil — все) и граммемы, одна из которых
// обязательна у точного разбора базового словаря (nil — без фильтра).
type profileSpec struct {
	kinds         []dicts.Kind
	baseGrammemes []string
}

var profiles = map[Profile]profileSpec{
	ProfileText: {},
	ProfileName: {
		kinds:         []dicts.Kind{dicts.KindBase, dicts.KindSurname, dicts.KindGiven, dicts.KindPatronymic},
		baseGrammemes: []string{"Name", "Surn", "Patr"},
	},
	ProfilePlace: {
		kinds:         []dicts.Kind{dicts.KindBase, dicts.KindToponym, dicts.KindAbbrev},
		baseGrammemes: []string{"Geox"},
	},
}
```

`internal/normalizer/lemma.go`:

```go
package normalizer

import "strings"

// Flag — признаки леммы (docs/search/normalization.md «Леммы»).
type Flag uint8

const (
	FlagAmbiguous Flag = 1 << iota // лемм у формы больше одной
	FlagPredicted                  // слова нет в словарях, лемма предсказана
	FlagUnknown                    // нет ни разбора, ни предсказания: лемма = форма
	FlagAbbrev                     // лемма из словаря сокращений
)

var flagNames = []struct {
	f    Flag
	name string
}{
	{FlagAmbiguous, "неоднозн"}, {FlagPredicted, "предск"}, {FlagUnknown, "неизв"}, {FlagAbbrev, "сокр"},
}

// String — признаки через запятую (для CLI и тестов).
func (f Flag) String() string {
	var out []string

	for _, n := range flagNames {
		if f&n.f != 0 {
			out = append(out, n.name)
		}
	}

	return strings.Join(out, ",")
}

// Lemma — нормальная форма слова с признаками.
type Lemma struct {
	Text  string
	Flags Flag
}
```

`internal/normalizer/term.go`:

```go
package normalizer

import "github.com/amarin/genodex/internal/textnorm"

// Term — слово для индекса или запроса: исходный токен, форма и леммы. Для
// части составного слова Form — сама часть, Token — всё слово.
type Term struct {
	Token  textnorm.Token
	Form   string
	Lemmas []Lemma
}
```

`internal/normalizer/tag.go`:

```go
package normalizer

import (
	"slices"
	"strings"

	"github.com/amarin/genodex/internal/dicts"
)

// stopPOS — служебные части речи: слово, у которого все точные разборы такие,
// в индекс и запрос не идёт.
var stopPOS = map[string]bool{"PREP": true, "CONJ": true, "PRCL": true, "INTJ": true}

// grammemes — граммемы тега gomorphy («NOUN,anim,masc sing,gent»).
func grammemes(tag string) []string {
	return strings.FieldsFunc(tag, func(r rune) bool { return r == ',' || r == ' ' })
}

// allStop — все разборы служебные.
func allStop(rs []dicts.Reading) bool {
	for _, r := range rs {
		g := grammemes(r.Tag)
		if len(g) == 0 || !stopPOS[g[0]] {
			return false
		}
	}

	return len(rs) > 0
}

// filterBase оставляет разборы небазовых словарей и базовые с одной из
// граммем профиля.
func filterBase(rs []dicts.Reading, want []string) []dicts.Reading {
	if want == nil {
		return rs
	}

	var out []dicts.Reading

	for _, r := range rs {
		if r.Kind != dicts.KindBase || slices.ContainsFunc(grammemes(r.Tag), func(g string) bool {
			return slices.Contains(want, g)
		}) {
			out = append(out, r)
		}
	}

	return out
}
```

`internal/normalizer/normalizer.go`:

```go
package normalizer

import (
	"strings"

	"github.com/amarin/genodex/internal/dicts"
	"github.com/amarin/genodex/internal/textnorm"
)

// Normalizer разбирает текст в термины с леммами.
type Normalizer struct {
	dicts Dictionaries
}

// New — нормализатор поверх словарей.
func New(d Dictionaries) *Normalizer { return &Normalizer{dicts: d} }

// Analyze — термины текста для индекса: стоп-слова отброшены, составное слово
// даёт термин целиком и по частям.
func (n *Normalizer) Analyze(text string, p Profile) []Term {
	var out []Term

	for _, tok := range textnorm.Tokenize(text) {
		out = append(out, n.terms(tok, profiles[p])...)
	}

	return out
}

// terms — термины одного токена.
func (n *Normalizer) terms(tok textnorm.Token, spec profileSpec) []Term {
	if tok.Number {
		return []Term{unknown(tok, tok.Form)}
	}

	if tok.Dotted {
		if ls := n.abbrev(tok.Form + "."); ls != nil {
			return []Term{{Token: tok, Form: tok.Form + ".", Lemmas: ls}}
		}
	}

	if !strings.Contains(tok.Form, "-") {
		if t, ok := n.word(tok, tok.Form, spec); ok {
			return []Term{t}
		}

		return nil
	}

	if ls := n.abbrev(tok.Form); ls != nil {
		return []Term{{Token: tok, Form: tok.Form, Lemmas: ls}}
	}

	var out []Term

	for _, form := range append([]string{tok.Form}, strings.Split(tok.Form, "-")...) {
		if form == "" {
			continue
		}

		if t, ok := n.word(tok, form, spec); ok {
			out = append(out, t)
		}
	}

	return out
}

// abbrev — леммы сокращения (только точные разборы словарей сокращений);
// nil — не сокращение.
func (n *Normalizer) abbrev(key string) []Lemma {
	exact, _ := n.parse(key, []dicts.Kind{dicts.KindAbbrev})

	return lemmas(exact, 0)
}

// word — термин слова: точные разборы (с дореформенными вариантами), иначе
// предсказанные, иначе неизвестное. false — стоп-слово.
func (n *Normalizer) word(tok textnorm.Token, form string, spec profileSpec) (Term, bool) {
	exact, predicted := n.parse(form, spec.kinds)
	if len(exact) == 0 {
		for _, v := range textnorm.ReformVariants(form) {
			if e, _ := n.parse(v, spec.kinds); len(e) > 0 {
				exact = e

				break
			}
		}
	}

	if allStop(exact) {
		return Term{}, false
	}

	ls := lemmas(filterBase(exact, spec.baseGrammemes), 0)
	if ls == nil {
		ls = lemmas(predicted, FlagPredicted)
	}

	if ls == nil {
		return unknown(tok, form), true
	}

	return Term{Token: tok, Form: form, Lemmas: ls}, true
}

// parse делит разборы на точные и предсказанные; предсказания словарей
// сокращений отбрасываются.
func (n *Normalizer) parse(form string, kinds []dicts.Kind) (exact, predicted []dicts.Reading) {
	for _, r := range n.dicts.Parse(form, kinds) {
		switch {
		case !r.Predicted:
			exact = append(exact, r)
		case r.Kind != dicts.KindAbbrev:
			predicted = append(predicted, r)
		}
	}

	return exact, predicted
}

// lemmas — различные леммы разборов (нормализованные как слово); у каждой —
// extra, FlagAbbrev для словаря сокращений, FlagAmbiguous при нескольких.
func lemmas(rs []dicts.Reading, extra Flag) []Lemma {
	var out []Lemma

	seen := map[string]bool{}

	for _, r := range rs {
		text := textnorm.NormalizeWord(r.Normal)
		if seen[text] {
			continue
		}

		seen[text] = true

		f := extra
		if r.Kind == dicts.KindAbbrev {
			f |= FlagAbbrev
		}

		out = append(out, Lemma{Text: text, Flags: f})
	}

	if len(out) > 1 {
		for i := range out {
			out[i].Flags |= FlagAmbiguous
		}
	}

	return out
}

// unknown — термин без разбора: лемма равна форме.
func unknown(tok textnorm.Token, form string) Term {
	return Term{Token: tok, Form: form, Lemmas: []Lemma{{Text: form, Flags: FlagUnknown}}}
}
```

Порядок в тесте «сокращение с точкой»: «с.» даёт «село», «сын» в порядке разборов словаря — порядок строк `abbrev.tsv` и `fake` совпадает.

- [ ] **Step 4: Моки и тесты**

Run: `go generate ./internal/normalizer/ && gofmt -l internal && go vet ./internal/normalizer/ && go test ./internal/normalizer/ -count=1`
Expected: создан `internal/normalizer/deps_test.go` (MockDictionaries), gofmt пусто, `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/normalizer/
git commit -m "feat(normalizer): разбор текста в термины — сокращения, леммы, профили полей, стоп-слова"
```

---

### Task 7: `normalizer` — разбор запроса

**Files:**
- Create: `internal/normalizer/query.go`
- Test: `internal/normalizer/query_test.go`

**Interfaces:**
- Consumes: `Normalizer.terms`, `profiles` (задача 6).
- Produces: `normalizer.Query{Terms []Term; Partial *textnorm.Token}`, `(*Normalizer).ParseQuery(q string, p Profile) Query`.

- [ ] **Step 1: Тест**

```go
package normalizer

import "testing"

// TestParseQuery: последнее слово без разделителя после него — незаконченное
// (префикс для подсказок P3/P4); число и слово с точкой законченные.
func TestParseQuery(t *testing.T) {
	cases := []struct {
		q, terms, partial string
	}{
		{"Кота Покро", "кота=кот[]", "покро"},
		{"Кота Покро ", "кота=кот[]; покро=покро[неизв]", ""},
		{"в Покро", "", "покро"},
		{"с.", "с.=село[неоднозн,сокр],сын[неоднозн,сокр]", ""},
		{"1834", "1834=1834[неизв]", ""},
		{"", "", ""},
	}
	n := newTest(t)

	for _, c := range cases {
		got := n.ParseQuery(c.q, ProfileText)

		partial := ""
		if got.Partial != nil {
			partial = got.Partial.Form
		}

		if show(got.Terms) != c.terms || partial != c.partial {
			t.Errorf("ParseQuery(%q) = %q / %q, want %q / %q", c.q, show(got.Terms), partial, c.terms, c.partial)
		}
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./internal/normalizer/ -run ParseQuery -count=1`
Expected: FAIL — `n.ParseQuery undefined`.

- [ ] **Step 3: Реализация**

`internal/normalizer/query.go`:

```go
package normalizer

import "github.com/amarin/genodex/internal/textnorm"

// Query — разобранный поисковый запрос.
type Query struct {
	Terms   []Term          // законченные слова (стоп-слова отброшены)
	Partial *textnorm.Token // незаконченное последнее слово — ищется префиксом по формам; nil — нет
}

// ParseQuery разбирает запрос тем же нормализатором, что и текст. Последнее
// слово, за которым нет ни одного символа (пользователь ещё печатает),
// считается незаконченным и не лемматизируется.
func (n *Normalizer) ParseQuery(q string, p Profile) Query {
	toks := textnorm.Tokenize(q)

	var out Query

	if k := len(toks); k > 0 {
		last := toks[k-1]
		if last.End == len(q) && !last.Number {
			out.Partial = &last
			toks = toks[:k-1]
		}
	}

	for _, tok := range toks {
		out.Terms = append(out.Terms, n.terms(tok, profiles[p])...)
	}

	return out
}
```

(Слово с точкой не бывает последним с `End == len(q)`: точка стоит после него.)

- [ ] **Step 4: Тесты проходят**

Run: `gofmt -l internal && go vet ./internal/normalizer/ && go test ./internal/normalizer/ -count=1`
Expected: gofmt пусто, `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/normalizer/query.go internal/normalizer/query_test.go
git commit -m "feat(normalizer): разбор запроса с незаконченным последним словом"
```

---

### Task 8: CLI `genodex analyze` / `genodex dicts` и подключение в приложение

**Files:**
- Create: `cmd/genodex/dicts.go`, `cmd/genodex/analyze.go`
- Modify: `cmd/genodex/main.go` (подкоманды, `--dicts` в `serve`), `internal/app/app.go` (`Config.DictsDir`, поле `dicts`, открытие, журнал, закрытие)
- Test: `cmd/genodex/analyze_test.go`

**Interfaces:**
- Consumes: `dicts.Open`, `dicts.Options`, `dicts.NewSQLStateStore`, `dicts.EmbeddedBase`, `(*Registry).List/SetEnabled/Version/Summary/Close`, `dicts.BaseName`, `basefetch.Fetch`, `normalizer.New`, `(*Normalizer).Analyze`, `normalizer.Profile*`, `storage.Open`.
- Produces: `app.Config.DictsDir string`; подкоманды `genodex dicts list|enable NAME|disable NAME|fetch`, `genodex analyze [--profile text|name|place] ТЕКСТ…`; флаг `--dicts DIR` у `serve`, `dicts`, `analyze` (по умолчанию `<data>/dicts`).

- [ ] **Step 1: Тест**

`cmd/genodex/analyze_test.go`:

```go
package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/amarin/genodex/internal/normalizer"
	"github.com/amarin/genodex/internal/textnorm"
)

func TestParseProfile(t *testing.T) {
	cases := map[string]normalizer.Profile{
		"text": normalizer.ProfileText, "name": normalizer.ProfileName, "place": normalizer.ProfilePlace,
	}
	for in, want := range cases {
		if got, err := parseProfile(in); err != nil || got != want {
			t.Errorf("parseProfile(%q) = %v, %v", in, got, err)
		}
	}

	if _, err := parseProfile("город"); err == nil {
		t.Error("parseProfile(город) без ошибки")
	}
}

func TestResolveDictsDir(t *testing.T) {
	if got := resolveDictsDir("", "/d"); got != filepath.Join("/d", "dicts") {
		t.Errorf("по умолчанию = %q", got)
	}

	if got := resolveDictsDir("/x", "/d"); got != "/x" {
		t.Errorf("явная папка = %q", got)
	}
}

func TestFormatTerms(t *testing.T) {
	var buf bytes.Buffer

	formatTerms(&buf, []normalizer.Term{
		{Token: textnorm.Token{Raw: "с"}, Form: "с.", Lemmas: []normalizer.Lemma{
			{Text: "село", Flags: normalizer.FlagAbbrev | normalizer.FlagAmbiguous},
			{Text: "сын", Flags: normalizer.FlagAbbrev | normalizer.FlagAmbiguous},
		}},
		{Token: textnorm.Token{Raw: "Кота"}, Form: "кота", Lemmas: []normalizer.Lemma{{Text: "кот"}}},
	})

	want := "с\tс.\tсело[неоднозн,сокр] сын[неоднозн,сокр]\nКота\tкота\tкот\n"
	if buf.String() != want {
		t.Fatalf("formatTerms =\n%q\nwant\n%q", buf.String(), want)
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./cmd/genodex/ -count=1`
Expected: FAIL — `undefined: parseProfile`, `resolveDictsDir`, `formatTerms`.

- [ ] **Step 3: Реализация**

`cmd/genodex/dicts.go`:

```go
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/amarin/genodex/internal/dicts"
	"github.com/amarin/genodex/internal/dicts/basefetch"
	"github.com/amarin/genodex/internal/storage"
)

// dictsDirFlag регистрирует --dicts; пусто — <data>/dicts.
func dictsDirFlag(fs *flag.FlagSet) *string {
	return fs.String("dicts", "", "папка словарей (по умолчанию <data>/dicts)")
}

// resolveDictsDir — явная папка словарей или <data>/dicts.
func resolveDictsDir(explicit, dataDir string) string {
	if explicit != "" {
		return explicit
	}

	return filepath.Join(dataDir, "dicts")
}

// openDicts открывает хранилище (состояние словарей) и реестр; close
// закрывает оба.
func openDicts(ctx context.Context, dataDir, dictsDir string) (*dicts.Registry, func(), error) {
	st, err := storage.Open(dataDir)
	if err != nil {
		return nil, nil, err
	}

	reg, err := dicts.Open(ctx, dicts.Options{
		Dir: dictsDir, Base: dicts.EmbeddedBase(), State: dicts.NewSQLStateStore(st.DB()),
	})
	if err != nil {
		_ = st.Close()

		return nil, nil, err
	}

	return reg, func() { _ = reg.Close(); _ = st.Close() }, nil
}

// runDicts — genodex dicts list|enable NAME|disable NAME|fetch.
func runDicts(ctx context.Context, preDataDir string, args []string) error {
	fs := newFlagSet("dicts")
	_ = dataDirFlag(fs)
	dir := dictsDirFlag(fs)

	if err := fs.Parse(args); err != nil {
		return err
	}

	dataDir := chooseDataDir(fs, preDataDir)
	dictsDir := resolveDictsDir(*dir, dataDir)

	switch fs.Arg(0) {
	case "fetch":
		dst := filepath.Join(dictsDir, dicts.BaseName+".dat")

		version, err := basefetch.Fetch(dst)
		if err != nil {
			return err
		}

		fmt.Printf("базовый словарь pymorphy2-dicts-ru %s → %s\n", version, dst)
		fmt.Println("изменение применится при следующем запуске сервера")

		return nil
	case "list", "enable", "disable":
	default:
		return errors.New("использование: genodex dicts list | enable ИМЯ | disable ИМЯ | fetch")
	}

	reg, closeFn, err := openDicts(ctx, dataDir, dictsDir)
	if err != nil {
		return err
	}
	defer closeFn()

	if fs.Arg(0) == "list" {
		return printDicts(reg)
	}

	if fs.NArg() != 2 {
		return fmt.Errorf("укажите имя словаря: genodex dicts %s ИМЯ", fs.Arg(0))
	}

	if err := reg.SetEnabled(ctx, fs.Arg(1), fs.Arg(0) == "enable"); err != nil {
		return err
	}

	fmt.Println("изменение применится при следующем запуске сервера")

	return nil
}

// printDicts — таблица словарей и версия набора.
func printDicts(reg *dicts.Registry) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ИМЯ\tВИД\tВКЛ\tОТКУДА\tИСТОЧНИК\tОШИБКА")

	for _, e := range reg.List() {
		on := "да"
		if !e.Enabled {
			on = "нет"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", e.Name, e.Kind, on, e.Origin, e.Info, e.Error)
	}

	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Printf("%s\nверсия набора: %s\n", reg.Summary(), reg.Version())

	return nil
}
```

`cmd/genodex/analyze.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/amarin/genodex/internal/normalizer"
)

// parseProfile — профиль разбора по имени флага.
func parseProfile(s string) (normalizer.Profile, error) {
	switch s {
	case "text":
		return normalizer.ProfileText, nil
	case "name":
		return normalizer.ProfileName, nil
	case "place":
		return normalizer.ProfilePlace, nil
	}

	return 0, fmt.Errorf("неизвестный профиль %q (text, name, place)", s)
}

// formatTerms — строка на термин: «как в тексте<TAB>форма<TAB>леммы».
func formatTerms(w io.Writer, terms []normalizer.Term) {
	for _, t := range terms {
		ls := make([]string, 0, len(t.Lemmas))

		for _, l := range t.Lemmas {
			if l.Flags == 0 {
				ls = append(ls, l.Text)

				continue
			}

			ls = append(ls, fmt.Sprintf("%s[%s]", l.Text, l.Flags))
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n", t.Token.Raw, t.Form, strings.Join(ls, " "))
	}
}

// runAnalyze — genodex analyze [--profile P] ТЕКСТ…: разбор текста
// нормализатором для проверки словарей.
func runAnalyze(ctx context.Context, preDataDir string, args []string) error {
	fs := newFlagSet("analyze")
	_ = dataDirFlag(fs)
	dir := dictsDirFlag(fs)
	profile := fs.String("profile", "text", "профиль разбора: text, name, place")

	if err := fs.Parse(args); err != nil {
		return err
	}

	p, err := parseProfile(*profile)
	if err != nil {
		return err
	}

	text := strings.Join(fs.Args(), " ")
	if text == "" {
		return errors.New("укажите текст: genodex analyze ТЕКСТ")
	}

	dataDir := chooseDataDir(fs, preDataDir)

	reg, closeFn, err := openDicts(ctx, dataDir, resolveDictsDir(*dir, dataDir))
	if err != nil {
		return err
	}
	defer closeFn()

	formatTerms(os.Stdout, normalizer.New(reg).Analyze(text, p))
	fmt.Printf("%s; версия набора: %s\n", reg.Summary(), reg.Version())

	return nil
}
```

`cmd/genodex/main.go`:

- в `run` список подкоманд: `case "serve", "backup", "restore", "verify", "dicts", "analyze":`;
- в `switch cmd` добавить:

```go
	case "dicts":
		return runDicts(ctx, preDataDir, rest)
	case "analyze":
		return runAnalyze(ctx, preDataDir, rest)
```

- в `runServe` после `trustProxy`: `dictsDir := dictsDirFlag(fs)`; в `app.Config{…}` вычислить каталог данных один раз и передать папку словарей:

```go
	dataDir := chooseDataDir(fs, preDataDir)

	a, err := app.New(app.Config{
		DataDir:    dataDir,
		DictsDir:   resolveDictsDir(*dictsDir, dataDir),
		Port:       *port,
		WebMode:    *webMode,
		TrustProxy: *trustProxy,
	})
```

`internal/app/app.go`:

- `Config` — поле `DictsDir string // папка словарей морфологии; "" — без папки`;
- `App` — поле `dicts *dicts.Registry`;
- в `New` сразу после успешного `sqlstore.Open`:

```go
	reg, err := dicts.Open(context.Background(), dicts.Options{
		Dir:   cfg.DictsDir,
		Base:  dicts.EmbeddedBase(),
		State: dicts.NewSQLStateStore(st.DB()),
	})
	if err != nil {
		_ = st.Close()

		return nil, fmt.Errorf("open dictionaries: %w", err)
	}
```

  и `dicts: reg` в литерал `&App{…}`;
- в `Run` после строки `Trust proxy`: `log.Printf("  Morphology: %s (версия набора %s)", a.dicts.Summary(), a.dicts.Version())`;
- в горутине остановки, перед `a.store.Close()`:

```go
		if err := a.dicts.Close(); err != nil {
			log.Printf("Dictionaries close error: %v", err)
		}
```

Импорты `internal/app`: `github.com/amarin/genodex/internal/dicts` (и `context`, если его ещё нет).

- [ ] **Step 4: Тесты и живая проверка**

Run:
```bash
gofmt -l . && go build ./... && go vet ./... && go test ./... -count=1
go test -tags nobasedict ./... -count=1
go run ./cmd/genodex --data "$(mktemp -d)" analyze "Кр-нин с. Покровскаго Ивановъ, 1834 г."
go run ./cmd/genodex --data "$(mktemp -d)" dicts list
```
Expected: всё зелёное в обоих режимах; `analyze` печатает `кр-нин → крестьянин[сокр]`, `с. → село/сын[неоднозн,сокр]`, `покровскаго → покровский`, `ивановъ` с формой `иванов`, `1834[неизв]`, `г. → город/год`; `dicts list` показывает `base.opencorpora` (встроенный, источник pymorphy2) и `abbrev.builtin`, `базовый: есть`. Фактический вывод занести в отчёт задачи.

- [ ] **Step 5: Commit**

```bash
git add cmd/genodex/ internal/app/app.go
git commit -m "feat(cli,app): genodex analyze и dicts, реестр словарей открывается при запуске сервера"
```

---

### Task 9: документация

**Files:**
- Modify: `AGENTS.md`, `docs/development.md`, `docs/usage.md`, `docs/search/normalization.md`, `docs/todo.md`

- [ ] **Step 1: `AGENTS.md`** — в «Main commands» после `go build ./...`:

```markdown
- `go generate ./internal/dicts/` — скачать и скомпилировать встроенный базовый словарь морфологии (`internal/dicts/base/base.opencorpora.dat`, генерат, нужен интернет). Без него `go build ./...` не собирается — как без `web/dist`; сборка без словаря — `-tags nobasedict`.
- `./genodex analyze [--profile text|name|place] ТЕКСТ` / `./genodex dicts list|enable ИМЯ|disable ИМЯ|fetch` — проверка нормализатора и управление словарями.
```

и в «Files that must not be edited»: `` `internal/dicts/base/*.dat` — generated; rebuild with `go generate ./internal/dicts/`. ``

- [ ] **Step 2: `docs/development.md`** — в таблицу бэкенда:

```markdown
| `go generate ./internal/dicts/` | Собрать встроенный базовый словарь морфологии (генерат, нужен интернет; `-force` у генератора — перекачать). Нужен для `go build` без тега `nobasedict`. |
| `go build -tags nobasedict ./...` | Сборка без встроенного базового словаря: поиск деградирует до точных форм и префиксов, словарь можно скачать позже `genodex dicts fetch`. |
```

и в «Что нельзя редактировать вручную» — `internal/dicts/base/*.dat`.

- [ ] **Step 3: `docs/usage.md`** — раздел «Словари морфологии»: папка `<data>/dicts` (флаг `--dicts`), имена файлов `<kind>.<name>.dat|tsv` и виды, формат TSV `лемма<TAB>словоформа[<TAB>теги]`, замена базового файлом `base.opencorpora.dat`, команды `genodex dicts …` и `genodex analyze …`, атрибуция: словарные данные OpenCorpora — CC BY-SA, источник виден в `genodex dicts list`. Изменения состояния из CLI применяются при следующем запуске сервера.

- [ ] **Step 4: `docs/search/normalization.md`** — синхронизировать с реализацией:
  - в таблицу орфографии — строку «латинская i/I внутри кириллического слова → и» и «U+2010 → дефис»;
  - в «Леммы» — абзац про дореформенные окончания: `-аго/-яго → -ого/-его`, `-ыя/-ия → -ые/-ой, -ие/-ей` пробуются, только если у формы нет точного разбора; форма в индексе остаётся как в тексте;
  - в «Папка словарей» — имена файлов `<kind>.<name>.dat|tsv`; состояние — таблица `dictionaries(name, enabled)`; порядок в P1 фиксирован (встроенные, затем файлы по алфавиту), настройка порядка — P7; сборка с `nobasedict` и `genodex dicts fetch`.

- [ ] **Step 5: `docs/todo.md`** — в P1 отметить `- [x] реализация`; в конец раздела P добавить:

```markdown
- [ ] Бэкап папки словарей: `genodex backup` сейчас копирует только SQLite,
      пользовательские словари `<data>/dicts` в бандл не попадают
```

- [ ] **Step 6: Проверка и commit**

Run: `gofmt -l . && go build ./... && go vet ./... && go test ./... -count=1`
Expected: зелёное.

```bash
git add AGENTS.md docs/
git commit -m "docs(search): P1 — сборка словарей, CLI, синхронизация normalization.md"
```

---

## Вне объёма P1

- Индекс, очередь, сверка по версии набора — P2 (версия уже доступна: `Registry.Version`).
- Словари, генерируемые из записей базы (`Surname`/`GivenName`/`Patronymic`), — вместе с очередью P2 (их пересборка — фоновое задание).
- Раскрытие вариантов по словарям базы («Авдотья» ← «Евдокия») — при поиске, P3.
- Смена состояния словарей на работающем сервере и настройка порядка — P7 (HTTP/UI); в P1 — CLI, применяется при следующем запуске.
- Бэкап папки словарей — пункт в `todo.md`.
