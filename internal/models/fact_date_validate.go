package models

// Validate проверяет инварианты даты: допустимые enum'ы, согласованность
// точности и компонентов, реальные границы месяцев и дней (с учётом
// календаря), верхнюю границу для between.
func (d FactDate) Validate() error {
	return finish("", d.validate())
}

// validate возвращает первую найденную ошибку или nil.
func (d FactDate) validate() *ValidationError {
	if !d.Precision.Valid() {
		return fieldErr("precision", "недопустимая точность %q", d.Precision)
	}
	if !d.Modifier.Valid() {
		return fieldErr("modifier", "недопустимая формулировка %q", d.Modifier)
	}
	if d.Calendar != "" && !d.Calendar.Valid() {
		return fieldErr("calendar", "недопустимый календарь %q", d.Calendar)
	}

	if d.Precision == PrecisionUnknown {
		for _, c := range []struct {
			field string
			value int
		}{
			{"year", d.Year}, {"month", d.Month}, {"day", d.Day},
			{"year_to", d.YearTo}, {"month_to", d.MonthTo}, {"day_to", d.DayTo},
		} {
			if c.value != 0 {
				return fieldErr(c.field, "при неизвестной точности компоненты даты должны быть нулевыми")
			}
		}
		if d.Modifier != ModifierExact {
			return fieldErr("modifier", "при неизвестной точности допустим только exact")
		}

		return nil
	}

	if d.Year < 1 || d.Year > 9999 {
		return fieldErr("year", "год вне диапазона 1–9999: %d", d.Year)
	}

	switch d.Precision {
	case PrecisionYear:
		if d.Month != 0 {
			return fieldErr("month", "при точности year месяц должен быть 0")
		}
		if d.Day != 0 {
			return fieldErr("day", "при точности year день должен быть 0")
		}
	case PrecisionMonth:
		if d.Month < 1 || d.Month > 12 {
			return fieldErr("month", "месяц вне диапазона 1–12: %d", d.Month)
		}
		if d.Day != 0 {
			return fieldErr("day", "при точности month день должен быть 0")
		}
	case PrecisionDay:
		if d.Month < 1 || d.Month > 12 {
			return fieldErr("month", "месяц вне диапазона 1–12: %d", d.Month)
		}
		if last := daysInMonth(d.Year, d.Month, d.Calendar); d.Day < 1 || d.Day > last {
			return fieldErr("day", "день вне диапазона 1–%d: %d", last, d.Day)
		}
	}

	return d.validateUpper()
}

// validateUpper проверяет верхнюю границу: она допустима только при
// modifier=between и не может заканчиваться раньше начала нижней.
func (d FactDate) validateUpper() *ValidationError {
	if d.Modifier != ModifierBetween {
		for _, c := range []struct {
			field string
			value int
		}{
			{"year_to", d.YearTo}, {"month_to", d.MonthTo}, {"day_to", d.DayTo},
		} {
			if c.value != 0 {
				return fieldErr(c.field, "верхняя граница допустима только при modifier=between")
			}
		}

		return nil
	}

	if d.YearTo < 1 || d.YearTo > 9999 {
		return fieldErr("year_to", "год верхней границы вне диапазона 1–9999: %d", d.YearTo)
	}
	if d.MonthTo != 0 && (d.MonthTo < 1 || d.MonthTo > 12) {
		return fieldErr("month_to", "месяц верхней границы вне диапазона 1–12: %d", d.MonthTo)
	}
	if d.DayTo != 0 {
		if d.MonthTo == 0 {
			return fieldErr("day_to", "день верхней границы задан без месяца")
		}
		if last := daysInMonth(d.YearTo, d.MonthTo, d.Calendar); d.DayTo < 1 || d.DayTo > last {
			return fieldErr("day_to", "день верхней границы вне диапазона 1–%d: %d", last, d.DayTo)
		}
	}

	// Конец верхней границы (неполные компоненты — до конца периода) не раньше
	// начала нижней (неполные компоненты — с начала периода).
	endMonth, endDay := d.MonthTo, d.DayTo
	if endMonth == 0 {
		endMonth = 12
	}
	if endDay == 0 {
		endDay = daysInMonth(d.YearTo, endMonth, d.Calendar)
	}
	startMonth, startDay := d.Month, d.Day
	if startMonth == 0 {
		startMonth = 1
	}
	if startDay == 0 {
		startDay = 1
	}
	if ord(d.YearTo, endMonth, endDay) < ord(d.Year, startMonth, startDay) {
		return fieldErr("year_to", "верхняя граница раньше нижней")
	}

	return nil
}

// daysInMonth — число дней месяца с учётом календаря. Григорианский — по
// григорианскому правилу високосности; юлианский и не заданный — по
// юлианскому (для не заданного календаря это снисходительное допущение).
func daysInMonth(year, month int, cal FactCalendar) int {
	if month == 2 {
		leap := year%4 == 0
		if cal == FactCalendarGregorian {
			leap = leap && (year%100 != 0 || year%400 == 0)
		}
		if leap {
			return 29
		}

		return 28
	}

	return julianMonthDays(year, month)
}

// validatePeriod проверяет период «с … по …»: обе даты (если заданы) корректны
// и начало не позже конца (по FactDate.Compare; неопределимое допускается).
func validatePeriod(since, until *FactDate) *ValidationError {
	if since != nil {
		if e := since.validate(); e != nil {
			return e.within("since")
		}
	}
	if until != nil {
		if e := until.validate(); e != nil {
			return e.within("until")
		}
	}
	if since != nil && until != nil && since.Compare(*until) == CompareLater {
		return fieldErr("since", "начало периода позже конца")
	}

	return nil
}
