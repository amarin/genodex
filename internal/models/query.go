package models

// Access — режим доступа читателя к данным. Нулевое значение — полный доступ
// (владелец); любое другое значение трактуется как публичный доступ, то есть
// фильтр приватного включается, а не отключается (безопасный отказ).
type Access int

const (
	// AccessFull — читатель видит всё, включая сущности с Private = true.
	AccessFull Access = iota
	// AccessPublic — сущности с Private = true скрыты. На сущности без флага
	// приватности (словари, церкви, приходы, деления) режим не влияет.
	AccessPublic
)

// Пределы страницы списка.
const (
	DefaultPageLimit = 50
	MaxPageLimit     = 500
)

// Page — окно списка: не больше Limit записей, начиная с Offset-й. Порядок
// записей стабилен и совпадает с порядком сохранения сущностей.
type Page struct {
	Limit, Offset int
}

// Normalized приводит окно к допустимому виду: Limit меньше либо равный нулю —
// DefaultPageLimit, больше MaxPageLimit — MaxPageLimit; отрицательный Offset — 0.
// Строгая проверка входных значений — дело обработчиков.
func (p Page) Normalized() Page {
	switch {
	case p.Limit <= 0:
		p.Limit = DefaultPageLimit
	case p.Limit > MaxPageLimit:
		p.Limit = MaxPageLimit
	}

	if p.Offset < 0 {
		p.Offset = 0
	}

	return p
}

// Hit — результат поиска: сущность, подпись для показа и поле, по которому
// найдено. Для одной сущности возвращается одна запись (первое совпавшее поле
// по алфавиту имён полей).
type Hit struct {
	Type  Type
	ID    ID
	Label string // подпись сущности: имя, название, заголовок; при пустой — ID
	Field string // поле поискового индекса: name, title, place, …
}

// DivisionKind — предметный вид единицы деления для фильтра списка.
type DivisionKind string

// DivisionKindSettlement — только населённые пункты (AdminDivisionType.IsSettlement).
const DivisionKindSettlement DivisionKind = "settlement"

// Valid сообщает, допустимо ли значение фильтра (пустое — «без фильтра»).
func (k DivisionKind) Valid() bool { return k == "" || k == DivisionKindSettlement }

// DivisionQuery — запрос списка единиц административного деления: необязательные
// фильтры по виду и типу (пересекаются), прямой родитель и окно. Окно применяется
// после фильтра. ParentID — список прямых детей единицы; nil — корень списка.
type DivisionQuery struct {
	Kind     DivisionKind      // "" — без фильтра; settlement — только населённые пункты
	Type     AdminDivisionType // "" — без фильтра; иначе точное совпадение типа
	ParentID *ID               // nil — корень; иначе только прямые дети этой единицы
	Page     Page
}

// Validate проверяет запрос: неизвестные вид и тип, отрицательные размер и
// сдвиг окна, неверный parent_id — *ValidationError (поля названы как параметры
// контракта). Размер окна больше MaxPageLimit не ошибка: Page.Normalized сужает его.
func (q DivisionQuery) Validate() error {
	switch {
	case !q.Kind.Valid():
		return fieldErr("kind", "неизвестный вид %q (допустимо: %q)", q.Kind, DivisionKindSettlement)
	case q.Type != "" && !q.Type.Valid():
		return fieldErr("type", "неизвестный тип единицы деления %q", q.Type)
	case q.Page.Limit < 0:
		return fieldErr("limit", "не может быть отрицательным: %d", q.Page.Limit)
	case q.Page.Offset < 0:
		return fieldErr("offset", "не может быть отрицательным: %d", q.Page.Offset)
	}

	if q.ParentID != nil {
		if err := idErr("parent_id", *q.ParentID, TypeAdministrativeDivision); err != nil {
			return err
		}
	}

	return nil
}

// Matches сообщает, проходит ли единица деления фильтры запроса.
func (q DivisionQuery) Matches(d AdministrativeDivision) bool {
	if q.Kind == DivisionKindSettlement && !d.Type.IsSettlement() {
		return false
	}

	return q.Type == "" || d.Type == q.Type
}

// DivisionSearchQuery — запрос поиска единиц административного деления по началу
// названия (включая варианты). Окно применяется после отбора единиц.
type DivisionSearchQuery struct {
	Text string // начало названия или варианта; пустое (после обрезки) — пустой результат
	Page Page
}

// Validate проверяет запрос: отрицательные размер и сдвиг окна — *ValidationError.
// Пустой текст не ошибка.
func (q DivisionSearchQuery) Validate() error {
	switch {
	case q.Page.Limit < 0:
		return fieldErr("limit", "не может быть отрицательным: %d", q.Page.Limit)
	case q.Page.Offset < 0:
		return fieldErr("offset", "не может быть отрицательным: %d", q.Page.Offset)
	}

	return nil
}

// SearchQuery — запрос поиска по началу названия/канонической формы (включая
// варианты), общий для сущностей без собственных доп. фильтров (см.
// DivisionSearchQuery для сущности с фильтрами). Окно применяется после
// отбора хитов own-типа.
type SearchQuery struct {
	Text string // начало текста; пустое (после обрезки) — пустой результат
	Page Page
}

// Validate проверяет запрос: отрицательные размер и сдвиг окна — *ValidationError.
// Пустой текст не ошибка.
func (q SearchQuery) Validate() error {
	switch {
	case q.Page.Limit < 0:
		return fieldErr("limit", "не может быть отрицательным: %d", q.Page.Limit)
	case q.Page.Offset < 0:
		return fieldErr("offset", "не может быть отрицательным: %d", q.Page.Offset)
	}

	return nil
}
