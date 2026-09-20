package models

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// FactPrecision — верхняя известная точность даты.
type FactPrecision string

const (
	PrecisionUnknown FactPrecision = "unknown"
	PrecisionYear    FactPrecision = "year"
	PrecisionMonth   FactPrecision = "month"
	PrecisionDay     FactPrecision = "day"
)

// FactModifier — вид формулировки даты.
type FactModifier string

const (
	ModifierExact   FactModifier = "exact"
	ModifierApprox  FactModifier = "approx"
	ModifierBefore  FactModifier = "before"
	ModifierAfter   FactModifier = "after"
	ModifierBetween FactModifier = "between"
)

// FactCompare — результат сравнения дат.
type FactCompare int

const (
	CompareEarlier       FactCompare = iota // раньше
	CompareLater                            // позже
	CompareIndeterminate                    // перекрытие / неопределимо
)

// FactDate — структурированная дата с точностью. Сравнение выполняется по
// компонентам даты, а не по строкам: парсинг формулировок происходит один раз
// при вводе.
type FactDate struct {
	// Year — год (обязателен, кроме precision=unknown).
	Year int
	// Month — месяц (0, если неизвестен).
	Month int
	// Day — день (0, если неизвестен).
	Day int
	// Precision — верхняя известная точность.
	Precision FactPrecision
	// Modifier — вид формулировки.
	Modifier FactModifier
	// Calendar — календарь, в котором записана дата (пусто ≡ unknown).
	Calendar FactCalendar
	// YearTo/MonthTo/DayTo — верхняя граница для modifier=between.
	YearTo  int
	MonthTo int
	DayTo   int
}

// UnknownDate возвращает дату неизвестной точности.
func UnknownDate() FactDate {
	return FactDate{Precision: PrecisionUnknown, Modifier: ModifierExact}
}

// Ordinal — монотонный порядковый номер компонентов даты для сравнения
// и построения границ интервалов.
type ordinal int

func ord(year, month, day int) ordinal {
	return ordinal(year*10000 + month*100 + day)
}

const (
	ordMin = ordinal(math.MinInt64)
	ordMax = ordinal(math.MaxInt64)
)

// bound представляет один конец интервала даты.
type bound struct {
	lo, hi ordinal
}

// bounds вычисляет интервал, покрываемый датой.
//
// Менее точная дата — диапазон на всю единицу точности: `1881` покрывает весь
// год, `1881-03` — весь месяц. При modifier=before/after интервал открыт
// с одной стороны; при between — от нижней границы до верхней (с точностью,
// определяемой наличием компонентов в to-полях).
func (d FactDate) bounds() (bound, bool) {
	if d.Precision == PrecisionUnknown {
		return bound{}, false
	}
	if d.Modifier == ModifierBetween {
		return d.betweenBounds(), true
	}
	lo, hi := d.singleBounds()
	switch d.Modifier {
	case ModifierBefore:
		return bound{ordMin, hi}, true
	case ModifierAfter:
		return bound{lo, ordMax}, true
	default:
		return bound{lo, hi}, true
	}
}

// singleBounds возвращает интервал одной точки/периода по точности.
func (d FactDate) singleBounds() (ordinal, ordinal) {
	switch d.Precision {
	case PrecisionDay:
		v := ord(d.Year, d.Month, d.Day)
		return v, v
	case PrecisionMonth:
		return ord(d.Year, d.Month, 1), ord(d.Year, d.Month, 31)
	default: // PrecisionYear
		return ord(d.Year, 1, 1), ord(d.Year, 12, 31)
	}
}

// betweenBounds возвращает интервал для modifier=between: нижняя граница —
// точно так же, как точка начала; верхняя определяется наличием to-компонентов.
func (d FactDate) betweenBounds() bound {
	lo, _ := d.singleBounds()
	hi := ord(d.YearTo, 1, 1)
	switch {
	case d.DayTo != 0:
		hi = ord(d.YearTo, d.MonthTo, d.DayTo)
	case d.MonthTo != 0:
		hi = ord(d.YearTo, d.MonthTo, 31)
	}
	return bound{lo, hi}
}

// Compare сравнивает даты по интервалам: раньше / позже / перекрытие
// (неопределимо). Если одна дата юлианская, а другая григорианская, границы
// юлианской переводятся в григорианские; в остальных случаях (одинаковые
// календари, unknown или пустой календарь) сравнение идёт «как есть».
func (d FactDate) Compare(other FactDate) FactCompare {
	a, okA := d.bounds()
	b, okB := other.bounds()
	if !okA || !okB {
		return CompareIndeterminate
	}
	switch {
	case d.Calendar == FactCalendarJulian && other.Calendar == FactCalendarGregorian:
		a = a.fromJulian()
	case d.Calendar == FactCalendarGregorian && other.Calendar == FactCalendarJulian:
		b = b.fromJulian()
	}
	switch {
	case a.hi < b.lo:
		return CompareEarlier
	case a.lo > b.hi:
		return CompareLater
	default:
		return CompareIndeterminate
	}
}

// julianToJDN возвращает юлианский день (JDN) для даты юлианского календаря.
func julianToJDN(y, m, d int) int {
	a := (14 - m) / 12
	yy := y + 4800 - a
	mm := m + 12*a - 3

	return d + (153*mm+2)/5 + 365*yy + yy/4 - 32083
}

// jdnToGregorian возвращает дату григорианского календаря по юлианскому дню.
func jdnToGregorian(jdn int) (year, month, day int) {
	a := jdn + 32044
	b := (4*a + 3) / 146097
	c := a - 146097*b/4
	d := (4*c + 3) / 1461
	e := c - 1461*d/4
	m := (5*e + 2) / 153

	day = e - (153*m+2)/5 + 1
	month = m + 3 - 12*(m/10)
	year = 100*b + d - 4800 + m/10

	return year, month, day
}

// julianMonthDays — число дней месяца в юлианском календаре (високосный год —
// каждый четвёртый).
func julianMonthDays(y, m int) int {
	switch m {
	case 2:
		if y%4 == 0 {
			return 29
		}

		return 28
	case 4, 6, 9, 11:
		return 30
	default:
		return 31
	}
}

// julianOrdToGregorian переводит порядковый номер юлианской даты в
// григорианский. Бесконечные границы не меняются; «31-е» верхней границы
// месяца сначала ограничивается реальным последним днём юлианского месяца.
func julianOrdToGregorian(o ordinal) ordinal {
	if o == ordMin || o == ordMax {
		return o
	}

	y, m, d := int(o)/10000, int(o)/100%100, int(o)%100
	if last := julianMonthDays(y, m); d > last {
		d = last
	}

	return ord(jdnToGregorian(julianToJDN(y, m, d)))
}

// fromJulian переводит обе границы интервала из юлианского календаря в григорианский.
func (b bound) fromJulian() bound {
	return bound{julianOrdToGregorian(b.lo), julianOrdToGregorian(b.hi)}
}

// SameYear сообщает, совпадает ли год обеих дат.
func (d FactDate) SameYear(other FactDate) bool {
	return d.Year != 0 && d.Year == other.Year
}

// SameMonth сообщает, совпадает ли месяц обеих дат.
func (d FactDate) SameMonth(other FactDate) bool {
	return d.Year != 0 && d.Month != 0 && d.Year == other.Year && d.Month == other.Month
}

// ParseFactDate разбирает строковую формулировку даты. Поддерживаемые формы:
// `1881`, `1881-03`, `1881-03-15`; с модификаторами `около`/`до`/`не позднее`/
// `после`/`не ранее`/`между X и Y`.
func ParseFactDate(s string) (FactDate, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return FactDate{}, fmt.Errorf("fact date: пустая строка")
	}
	s, cal := splitCalendarSuffix(s)

	mod := ModifierExact
	lower := strings.ToLower(s)
	rest := s
	between := false
	switch {
	case strings.HasPrefix(lower, "между "):
		between = true
		mod = ModifierBetween
		rest = strings.TrimSpace(s[len("между "):])
	case strings.HasPrefix(lower, "не ранее "):
		mod = ModifierAfter
		rest = strings.TrimSpace(s[len("не ранее "):])
	case strings.HasPrefix(lower, "не позднее "):
		mod = ModifierBefore
		rest = strings.TrimSpace(s[len("не позднее "):])
	case strings.HasPrefix(lower, "около "):
		mod = ModifierApprox
		rest = strings.TrimSpace(s[len("около "):])
	case strings.HasPrefix(lower, "после "):
		mod = ModifierAfter
		rest = strings.TrimSpace(s[len("после "):])
	case strings.HasPrefix(lower, "до "):
		mod = ModifierBefore
		rest = strings.TrimSpace(s[len("до "):])
	}

	if between {
		parts := strings.Split(rest, " и ")
		if len(parts) != 2 {
			return FactDate{}, fmt.Errorf("fact date: ожидается «между X и Y», получено %q", s)
		}
		lo, err := parseComponents(strings.TrimSpace(parts[0]))
		if err != nil {
			return FactDate{}, err
		}
		hi, err := parseComponents(strings.TrimSpace(parts[1]))
		if err != nil {
			return FactDate{}, err
		}
		return FactDate{
			Year: lo.year, Month: lo.month, Day: lo.day, Precision: lo.precision,
			Modifier: mod, Calendar: cal,
			YearTo: hi.year, MonthTo: hi.month, DayTo: hi.day,
		}, nil
	}

	c, err := parseComponents(rest)
	if err != nil {
		return FactDate{}, err
	}
	return FactDate{
		Year: c.year, Month: c.month, Day: c.day, Precision: c.precision,
		Modifier: mod, Calendar: cal,
	}, nil
}

// calendarSuffixes — суффиксы календаря в конце строки даты (в нижнем регистре).
// Порядок важен только в пределах одного календаря.
var calendarSuffixes = []struct {
	suffix   string
	calendar FactCalendar
}{
	{"ст. ст.", FactCalendarJulian},
	{"ст.ст.", FactCalendarJulian},
	{"н. ст.", FactCalendarGregorian},
	{"н.ст.", FactCalendarGregorian},
}

// splitCalendarSuffix отделяет суффикс календаря: «ст. ст.» — юлианский
// (старый стиль), «н. ст.» — григорианский (новый стиль). Регистр не важен;
// без суффикса календарь пуст. Суффиксы состоят из букв, не меняющих длину в
// байтах при смене регистра, поэтому усечение по длине безопасно.
func splitCalendarSuffix(s string) (string, FactCalendar) {
	lower := strings.ToLower(s)
	for _, sf := range calendarSuffixes {
		if strings.HasSuffix(lower, sf.suffix) {
			return strings.TrimSpace(s[:len(s)-len(sf.suffix)]), sf.calendar
		}
	}

	return s, ""
}

// components — результат разбора числовых компонентов даты.
type components struct {
	year, month, day int
	precision        FactPrecision
}

// parseComponents разбирает `1881`, `1881-03`, `1881-03-15`.
func parseComponents(s string) (components, error) {
	if s == "" {
		return components{}, fmt.Errorf("fact date: пустые компоненты даты")
	}
	parts := strings.Split(s, "-")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return components{}, fmt.Errorf("fact date: неверный компонент даты %q", p)
		}
		nums = append(nums, n)
	}
	if len(nums) > 3 {
		return components{}, fmt.Errorf("fact date: слишком много компонентов даты %q", s)
	}
	year := nums[0]
	if year < 1 || year > 9999 {
		return components{}, fmt.Errorf("fact date: год вне диапазона: %d", year)
	}
	c := components{year: year}
	switch len(nums) {
	case 1:
		c.precision = PrecisionYear
	case 2:
		if nums[1] < 1 || nums[1] > 12 {
			return components{}, fmt.Errorf("fact date: неверный месяц: %d", nums[1])
		}
		c.month, c.precision = nums[1], PrecisionMonth
	case 3:
		if nums[1] < 1 || nums[1] > 12 {
			return components{}, fmt.Errorf("fact date: неверный месяц: %d", nums[1])
		}
		if nums[2] < 1 || nums[2] > 31 {
			return components{}, fmt.Errorf("fact date: неверный день: %d", nums[2])
		}
		c.month, c.day, c.precision = nums[1], nums[2], PrecisionDay
	}
	return c, nil
}

// String возвращает каноническую текстовую форму даты; для юлианского и
// григорианского календаря добавляется суффикс « ст. ст.» / « н. ст.».
func (d FactDate) String() string {
	s := d.text()
	if d.Precision == PrecisionUnknown {
		return s
	}
	switch d.Calendar {
	case FactCalendarJulian:
		return s + " ст. ст."
	case FactCalendarGregorian:
		return s + " н. ст."
	}

	return s
}

// text возвращает форму даты без суффикса календаря.
func (d FactDate) text() string {
	if d.Precision == PrecisionUnknown {
		return "?"
	}
	var base string
	switch d.Precision {
	case PrecisionDay:
		base = fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
	case PrecisionMonth:
		base = fmt.Sprintf("%04d-%02d", d.Year, d.Month)
	default:
		base = fmt.Sprintf("%04d", d.Year)
	}
	switch d.Modifier {
	case ModifierApprox:
		return "около " + base
	case ModifierBefore:
		return "до " + base
	case ModifierAfter:
		return "после " + base
	case ModifierBetween:
		hi := fmt.Sprintf("%04d", d.YearTo)
		switch {
		case d.DayTo != 0:
			hi = fmt.Sprintf("%04d-%02d-%02d", d.YearTo, d.MonthTo, d.DayTo)
		case d.MonthTo != 0:
			hi = fmt.Sprintf("%04d-%02d", d.YearTo, d.MonthTo)
		}
		return "между " + base + " и " + hi
	default:
		return base
	}
}
