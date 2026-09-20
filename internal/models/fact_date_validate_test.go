package models

import "testing"

func TestFactDateValidate(t *testing.T) {
	day := FactDate{Year: 1881, Month: 3, Day: 15, Precision: PrecisionDay, Modifier: ModifierExact}
	with := func(f func(*FactDate)) FactDate {
		d := day
		f(&d)

		return d
	}

	valid := map[string]FactDate{
		"день":                     day,
		"месяц":                    {Year: 1881, Month: 3, Precision: PrecisionMonth, Modifier: ModifierExact},
		"год":                      {Year: 1881, Precision: PrecisionYear, Modifier: ModifierApprox},
		"неизвестная":              UnknownDate(),
		"юлианский 29.02.1900":     {Year: 1900, Month: 2, Day: 29, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarJulian},
		"без календаря 29.02.1900": {Year: 1900, Month: 2, Day: 29, Precision: PrecisionDay, Modifier: ModifierExact},
		"between по годам":         {Year: 1880, Precision: PrecisionYear, Modifier: ModifierBetween, YearTo: 1882},
		"between год внутри":       {Year: 1880, Month: 3, Precision: PrecisionMonth, Modifier: ModifierBetween, YearTo: 1880},
		"between по дням":          {Year: 1880, Month: 3, Day: 1, Precision: PrecisionDay, Modifier: ModifierBetween, YearTo: 1880, MonthTo: 3, DayTo: 31},
		"до":                       {Year: 1881, Precision: PrecisionYear, Modifier: ModifierBefore},
	}
	for name, d := range valid {
		if err := d.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	invalid := []struct {
		name  string
		d     FactDate
		field string
	}{
		{"пустая точность", with(func(d *FactDate) { d.Precision = "" }), "precision"},
		{"пустая формулировка", with(func(d *FactDate) { d.Modifier = "" }), "modifier"},
		{"плохой календарь", with(func(d *FactDate) { d.Calendar = "mayan" }), "calendar"},
		{"unknown с годом", FactDate{Year: 1881, Precision: PrecisionUnknown, Modifier: ModifierExact}, "year"},
		{"unknown с YearTo", FactDate{Precision: PrecisionUnknown, Modifier: ModifierExact, YearTo: 1900}, "year_to"},
		{"unknown с approx", FactDate{Precision: PrecisionUnknown, Modifier: ModifierApprox}, "modifier"},
		{"год 0", with(func(d *FactDate) { d.Year = 0 }), "year"},
		{"год 10000", with(func(d *FactDate) { d.Year = 10000 }), "year"},
		{"год с месяцем", FactDate{Year: 1881, Month: 3, Precision: PrecisionYear, Modifier: ModifierExact}, "month"},
		{"год с днём", FactDate{Year: 1881, Day: 3, Precision: PrecisionYear, Modifier: ModifierExact}, "day"},
		{"месяц 13", FactDate{Year: 1881, Month: 13, Precision: PrecisionMonth, Modifier: ModifierExact}, "month"},
		{"месяц 0", FactDate{Year: 1881, Precision: PrecisionMonth, Modifier: ModifierExact}, "month"},
		{"месяц с днём", FactDate{Year: 1881, Month: 3, Day: 5, Precision: PrecisionMonth, Modifier: ModifierExact}, "day"},
		{"день 0", with(func(d *FactDate) { d.Day = 0 }), "day"},
		{"31 апреля", with(func(d *FactDate) { d.Month, d.Day = 4, 31 }), "day"},
		{"29.02.1900 григорианский", with(func(d *FactDate) { d.Month, d.Day, d.Year, d.Calendar = 2, 29, 1900, FactCalendarGregorian }), "day"},
		{"29.02.1901 юлианский", with(func(d *FactDate) { d.Month, d.Day, d.Year, d.Calendar = 2, 29, 1901, FactCalendarJulian }), "day"},
		{"YearTo без between", with(func(d *FactDate) { d.YearTo = 1900 }), "year_to"},
		{"MonthTo без between", with(func(d *FactDate) { d.MonthTo = 5 }), "month_to"},
		{"between без YearTo", FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierBetween}, "year_to"},
		{"between DayTo без MonthTo", FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierBetween, YearTo: 1882, DayTo: 3}, "day_to"},
		{"between MonthTo 13", FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierBetween, YearTo: 1882, MonthTo: 13}, "month_to"},
		{"between DayTo 31 февраля", FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierBetween, YearTo: 1882, MonthTo: 2, DayTo: 31}, "day_to"},
		{"between верхняя раньше нижней", FactDate{Year: 1900, Precision: PrecisionYear, Modifier: ModifierBetween, YearTo: 1899}, "year_to"},
		{"between верхний месяц раньше нижнего", FactDate{Year: 1900, Month: 6, Precision: PrecisionMonth, Modifier: ModifierBetween, YearTo: 1900, MonthTo: 3}, "year_to"},
	}
	for _, tt := range invalid {
		wantInvalid(t, tt.name, tt.d.Validate(), "", tt.field)
	}
}

func TestDaysInMonth(t *testing.T) {
	tests := []struct {
		year, month int
		cal         FactCalendar
		want        int
	}{
		{1900, 2, FactCalendarGregorian, 28},
		{1900, 2, FactCalendarJulian, 29},
		{1900, 2, "", 29}, // календарь не задан — снисходительно, по юлианскому правилу
		{2000, 2, FactCalendarGregorian, 29},
		{1901, 2, FactCalendarJulian, 28},
		{1881, 4, FactCalendarGregorian, 30},
		{1881, 3, FactCalendarJulian, 31},
	}
	for _, tt := range tests {
		if got := daysInMonth(tt.year, tt.month, tt.cal); got != tt.want {
			t.Errorf("daysInMonth(%d, %d, %q) = %d, want %d", tt.year, tt.month, tt.cal, got, tt.want)
		}
	}
}

func TestValidatePeriod(t *testing.T) {
	y := func(year int) *FactDate {
		return &FactDate{Year: year, Precision: PrecisionYear, Modifier: ModifierExact}
	}

	if e := validatePeriod(nil, nil); e != nil {
		t.Errorf("пустой период: %v", e)
	}
	if e := validatePeriod(y(1881), nil); e != nil {
		t.Errorf("открытый конец: %v", e)
	}
	if e := validatePeriod(y(1881), y(1881)); e != nil {
		t.Errorf("совпадающие даты (неопределимо) допустимы: %v", e)
	}
	if e := validatePeriod(y(1881), y(1900)); e != nil {
		t.Errorf("нормальный период: %v", e)
	}

	wantInvalid(t, "начало позже конца", finish("", validatePeriod(y(1900), y(1881))), "", "since")
	wantInvalid(t, "плохое начало", finish("", validatePeriod(&FactDate{}, nil)), "", "since.precision")
	wantInvalid(t, "плохой конец", finish("", validatePeriod(nil, &FactDate{Year: 1881, Precision: PrecisionYear})), "", "until.modifier")

	// Начало 25.10.1917 ст. ст. не позже 07.11.1917 н. ст. (один день).
	julian := &FactDate{Year: 1917, Month: 10, Day: 25, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarJulian}
	greg := &FactDate{Year: 1917, Month: 11, Day: 7, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarGregorian}
	if e := validatePeriod(julian, greg); e != nil {
		t.Errorf("период в разных календарях: %v", e)
	}
	wantInvalid(t, "разные календари, начало позже", finish("", validatePeriod(greg, &FactDate{Year: 1917, Month: 10, Day: 24, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarJulian})), "", "since")
}
