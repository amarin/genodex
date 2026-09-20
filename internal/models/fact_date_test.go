package models

import "testing"

func TestParseFactDate(t *testing.T) {
	tests := []struct {
		in   string
		want FactDate
	}{
		{"1881", FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierExact}},
		{"около 1881", FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierApprox}},
		{"1881-03", FactDate{Year: 1881, Month: 3, Precision: PrecisionMonth, Modifier: ModifierExact}},
		{"1881-03-15", FactDate{Year: 1881, Month: 3, Day: 15, Precision: PrecisionDay, Modifier: ModifierExact}},
		{"до 1881", FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierBefore}},
		{"не позднее 1881", FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierBefore}},
		{"после 1881", FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierAfter}},
		{"не ранее 1881", FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierAfter}},
		{"между 1880 и 1882", FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierBetween, YearTo: 1882}},
		{"  1881  ", FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierExact}},
		{"между 1880-03 и 1882-09-01", FactDate{Year: 1880, Month: 3, Precision: PrecisionMonth, Modifier: ModifierBetween, YearTo: 1882, MonthTo: 9, DayTo: 1}},
	}
	for _, tt := range tests {
		got, err := ParseFactDate(tt.in)
		if err != nil {
			t.Errorf("ParseFactDate(%q): unexpected error %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseFactDate(%q)=%+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestParseFactDateErrors(t *testing.T) {
	for _, in := range []string{"", "abc", "1881-13", "1881-00", "1881-03-32", "между 1880", "между x и 1882", "0"} {
		if _, err := ParseFactDate(in); err == nil {
			t.Errorf("ParseFactDate(%q): want error, got nil", in)
		}
	}
}

func TestFactDateString(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"1881", "1881"},
		{"около 1881", "около 1881"},
		{"1881-03", "1881-03"},
		{"1881-03-15", "1881-03-15"},
		{"до 1881", "до 1881"},
		{"после 1881", "после 1881"},
		{"между 1880 и 1882", "между 1880 и 1882"},
		{"между 1880-03 и 1882-09-01", "между 1880-03 и 1882-09-01"},
	}
	for _, tt := range tests {
		d, err := ParseFactDate(tt.in)
		if err != nil {
			t.Errorf("parse %q: %v", tt.in, err)
			continue
		}
		if got := d.String(); got != tt.want {
			t.Errorf("String(%q)=%q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFactDateCompare(t *testing.T) {
	must := func(s string) FactDate {
		d, err := ParseFactDate(s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		return d
	}

	tests := []struct {
		a, b string
		want FactCompare
	}{
		{"1881", "1881", CompareIndeterminate},              // перекрытие
		{"1881", "1882", CompareEarlier},                    // раньше
		{"1882", "1881", CompareLater},                      // позже
		{"1881-03", "1881-03-15", CompareIndeterminate},     // точка внутри месяца
		{"1881-03-01", "1881-03-15", CompareEarlier},        // точка vs точка
		{"до 1881", "1882", CompareEarlier},                 // интервал целиком раньше
		{"до 1881", "1880", CompareIndeterminate},           // 1880 попадает в «до 1881»
		{"после 1881", "1880", CompareLater},                // интервал целиком позже
		{"после 1881", "1882", CompareIndeterminate},        // 1882 попадает в «после 1881»
		{"между 1880 и 1882", "1879", CompareLater},         // интервал целиком позже
		{"между 1880 и 1882", "1881", CompareIndeterminate}, // внутри интервала
		{"между 1880 и 1882", "1883", CompareEarlier},       // интервал целиком раньше
	}
	for _, tt := range tests {
		a, b := must(tt.a), must(tt.b)
		if got := a.Compare(b); got != tt.want {
			t.Errorf("Compare(%q, %q)=%v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestFactDateUnknownCompare(t *testing.T) {
	u := UnknownDate()
	if u.Precision != PrecisionUnknown {
		t.Errorf("Unknown().Precision=%v", u.Precision)
	}
	d, _ := ParseFactDate("1881")
	if u.Compare(d) != CompareIndeterminate || d.Compare(u) != CompareIndeterminate {
		t.Errorf("compare with unknown: want indeterminate")
	}
}

func TestFactDateSameYearMonth(t *testing.T) {
	d1881, _ := ParseFactDate("1881-03")
	d1881b, _ := ParseFactDate("1881-07")
	d1882, _ := ParseFactDate("1882-03")
	dNoMonth, _ := ParseFactDate("1881")

	if !d1881.SameYear(d1881b) {
		t.Errorf("SameYear want true")
	}
	if d1881.SameYear(d1882) {
		t.Errorf("SameYear want false")
	}
	if d1881.SameMonth(d1881b) {
		t.Errorf("SameMonth(03, 07) want false")
	}
	if !d1881.SameMonth(mustParse(t, "1881-03-15")) {
		t.Errorf("SameMonth with month precision want true")
	}
	if d1881.SameMonth(dNoMonth) {
		t.Errorf("SameMonth with year-only want false")
	}
}

func mustParse(t *testing.T, s string) FactDate {
	t.Helper()
	d, err := ParseFactDate(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return d
}

func TestParseFactDateCalendar(t *testing.T) {
	tests := []struct {
		in   string
		want FactDate
	}{
		{"1917-10-25 ст. ст.", FactDate{Year: 1917, Month: 10, Day: 25, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarJulian}},
		{"1917-10-25 ст.ст.", FactDate{Year: 1917, Month: 10, Day: 25, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarJulian}},
		{"1917-11-07 н. ст.", FactDate{Year: 1917, Month: 11, Day: 7, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarGregorian}},
		{"1917-11-07 н.ст.", FactDate{Year: 1917, Month: 11, Day: 7, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarGregorian}},
		{"1917 Ст. Ст.", FactDate{Year: 1917, Precision: PrecisionYear, Modifier: ModifierExact, Calendar: FactCalendarJulian}},
		{"около 1881 ст. ст.", FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierApprox, Calendar: FactCalendarJulian}},
		{"между 1880 и 1882 ст. ст.", FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierBetween, YearTo: 1882, Calendar: FactCalendarJulian}},
		{"  1881-03  н. ст.  ", FactDate{Year: 1881, Month: 3, Precision: PrecisionMonth, Modifier: ModifierExact, Calendar: FactCalendarGregorian}},
	}
	for _, tt := range tests {
		got, err := ParseFactDate(tt.in)
		if err != nil {
			t.Errorf("ParseFactDate(%q): unexpected error %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseFactDate(%q)=%+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestParseFactDateCalendarErrors(t *testing.T) {
	for _, in := range []string{"ст. ст.", "н. ст.", "между 1880 ст. ст."} {
		if _, err := ParseFactDate(in); err == nil {
			t.Errorf("ParseFactDate(%q): want error, got nil", in)
		}
	}
}

func TestFactDateStringCalendar(t *testing.T) {
	tests := []struct {
		in   FactDate
		want string
	}{
		{FactDate{Year: 1917, Month: 10, Day: 25, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarJulian}, "1917-10-25 ст. ст."},
		{FactDate{Year: 1917, Month: 11, Day: 7, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarGregorian}, "1917-11-07 н. ст."},
		{FactDate{Year: 1917, Precision: PrecisionYear, Modifier: ModifierApprox, Calendar: FactCalendarJulian}, "около 1917 ст. ст."},
		{FactDate{Year: 1917, Precision: PrecisionYear, Modifier: ModifierExact, Calendar: FactCalendarUnknown}, "1917"},
		{FactDate{Year: 1917, Precision: PrecisionYear, Modifier: ModifierExact}, "1917"},
		{FactDate{Precision: PrecisionUnknown, Modifier: ModifierExact, Calendar: FactCalendarJulian}, "?"},
	}
	for _, tt := range tests {
		if got := tt.in.String(); got != tt.want {
			t.Errorf("String(%+v)=%q, want %q", tt.in, got, tt.want)
		}
	}
}

// Разбор и вывод обратимы для всех форм с календарём.
func TestFactDateCalendarRoundTrip(t *testing.T) {
	for _, in := range []string{
		"1917-10-25 ст. ст.",
		"1917-11-07 н. ст.",
		"около 1881 ст. ст.",
		"между 1880-03 и 1882-09-01 ст. ст.",
		"до 1918 н. ст.",
	} {
		d, err := ParseFactDate(in)
		if err != nil {
			t.Errorf("parse %q: %v", in, err)
			continue
		}
		if got := d.String(); got != in {
			t.Errorf("String(Parse(%q))=%q", in, got)
		}
	}
}

func TestFactDateCompareCalendars(t *testing.T) {
	tests := []struct {
		a, b string
		want FactCompare
	}{
		// 25 октября 1917 ст. ст. — это 7 ноября 1917 н. ст.: один и тот же день.
		{"1917-10-25 ст. ст.", "1917-11-07 н. ст.", CompareIndeterminate},
		{"1917-11-07 н. ст.", "1917-10-25 ст. ст.", CompareIndeterminate},
		// Без пересчёта 25 октября было бы «раньше» 6 ноября — пересчёт это исправляет.
		{"1917-10-25 ст. ст.", "1917-11-06 н. ст.", CompareLater},
		{"1917-11-06 н. ст.", "1917-10-25 ст. ст.", CompareEarlier},
		{"1917-10-25 ст. ст.", "1917-11-08 н. ст.", CompareEarlier},
		// Юлианский месяц покрывает 14 января – 13 февраля 1918 н. ст.
		{"1918-01 ст. ст.", "1918-02-01 н. ст.", CompareIndeterminate},
		{"1918-01 ст. ст.", "1918-02-14 н. ст.", CompareEarlier},
		{"1918-01 ст. ст.", "1918-01-13 н. ст.", CompareLater},
		// 1900 — високосный год в юлианском календаре: 29 февраля ст. ст. = 13 марта н. ст.
		{"1900-02-29 ст. ст.", "1900-03-13 н. ст.", CompareIndeterminate},
		{"1900-02-29 ст. ст.", "1900-03-12 н. ст.", CompareLater},
		// Границы «до»/«после» остаются бесконечными и после пересчёта.
		{"до 1917-10-25 ст. ст.", "1917-11-08 н. ст.", CompareEarlier},
		{"после 1917-10-25 ст. ст.", "1917-11-06 н. ст.", CompareLater},
		{"после 1917-10-25 ст. ст.", "1917-11-08 н. ст.", CompareIndeterminate},
		// Календарь неизвестен или совпадает — сравнение «как есть».
		{"1917-10-25 ст. ст.", "1917-11-06", CompareEarlier},
		{"1917-10-25", "1917-11-06 н. ст.", CompareEarlier},
		{"1917-10-25 ст. ст.", "1917-11-06 ст. ст.", CompareEarlier},
		{"1917-10-25 н. ст.", "1917-11-06 н. ст.", CompareEarlier},
		// Обрезка длины месяца: юлианский февраль 1900 (29 дней) кончается 13 марта н. ст.
		{"1900-02 ст. ст.", "1900-03-14 н. ст.", CompareEarlier},
		{"1900-03-14 н. ст.", "1900-02 ст. ст.", CompareLater},
		{"1900-02 ст. ст.", "1900-03-13 н. ст.", CompareIndeterminate},
		{"1900-03-13 н. ст.", "1900-02 ст. ст.", CompareIndeterminate},
	}
	for _, tt := range tests {
		a, b := mustParse(t, tt.a), mustParse(t, tt.b)
		if got := a.Compare(b); got != tt.want {
			t.Errorf("Compare(%q, %q)=%v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

// Явный календарь unknown подавляет пересчёт: сравнение идёт «как есть».
func TestFactDateCompareUnknownCalendarSuppressesConversion(t *testing.T) {
	unknown := func(s string) FactDate {
		d := mustParse(t, s)
		d.Calendar = FactCalendarUnknown
		return d
	}
	if got := unknown("1917-10-25").Compare(mustParse(t, "1917-11-06 н. ст.")); got != CompareEarlier {
		t.Errorf("unknown vs н. ст.: got %v, want CompareEarlier", got)
	}
	if got := mustParse(t, "1917-10-25 ст. ст.").Compare(unknown("1917-11-06")); got != CompareEarlier {
		t.Errorf("ст. ст. vs unknown: got %v, want CompareEarlier", got)
	}
}

func TestFactDateCompareBetweenYearUpperBound(t *testing.T) {
	tests := []struct {
		a, b string
		want FactCompare
	}{
		// Верхняя граница «между 1899 и 1901» — 31 декабря 1901.
		{"между 1899 и 1901", "1901-06-01", CompareIndeterminate},
		{"1901-06-01", "между 1899 и 1901", CompareIndeterminate},
		{"между 1899 и 1901", "1902-01-01", CompareEarlier},
		{"1902-01-01", "между 1899 и 1901", CompareLater},
		{"между 1899 и 1901", "1898-12-31", CompareLater},
		{"1898-12-31", "между 1899 и 1901", CompareEarlier},
		// 1900-12-27 ст. ст. = 1901-01-09 н. ст., внутри интервала.
		{"между 1899 и 1901 н. ст.", "1900-12-27 ст. ст.", CompareIndeterminate},
		{"1900-12-27 ст. ст.", "между 1899 и 1901 н. ст.", CompareIndeterminate},
	}
	for _, tt := range tests {
		a, b := mustParse(t, tt.a), mustParse(t, tt.b)
		if got := a.Compare(b); got != tt.want {
			t.Errorf("Compare(%q, %q)=%v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestJulianToGregorian(t *testing.T) {
	tests := []struct {
		y, m, d          int
		wantY, wantM, wD int
	}{
		{1917, 10, 25, 1917, 11, 7},
		{1918, 1, 31, 1918, 2, 13},
		{1900, 2, 29, 1900, 3, 13},
		{1582, 10, 4, 1582, 10, 14},
		{1700, 2, 28, 1700, 3, 10},
		{1800, 1, 1, 1800, 1, 12}, // до 1 марта 1800 разница ещё 11 дней
	}
	for _, tt := range tests {
		gy, gm, gd := jdnToGregorian(julianToJDN(tt.y, tt.m, tt.d))
		if gy != tt.wantY || gm != tt.wantM || gd != tt.wD {
			t.Errorf("Julian %d-%02d-%02d → Gregorian %d-%02d-%02d, want %d-%02d-%02d",
				tt.y, tt.m, tt.d, gy, gm, gd, tt.wantY, tt.wantM, tt.wD)
		}
	}
}
