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
