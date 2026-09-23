package transport

import "github.com/amarin/genodex/internal/models"

// FactDate — контракт структурированной даты с точностью (models.FactDate):
// год/месяц/день, верхняя известная точность, формулировка (точно/около/до/
// после/между), календарь (юлианский/григорианский/неизвестен/не указан),
// верхняя граница периода для modifier=between. nil — дата не указана.
type FactDate struct {
	Year      int    `json:"year"`
	Month     int    `json:"month,omitempty"`
	Day       int    `json:"day,omitempty"`
	Precision string `json:"precision"`
	Modifier  string `json:"modifier"`
	Calendar  string `json:"calendar,omitempty"`
	YearTo    int    `json:"year_to,omitempty"`
	MonthTo   int    `json:"month_to,omitempty"`
	DayTo     int    `json:"day_to,omitempty"`
}

// FactDateFromModel конвертирует дату в контракт; nil — дата не указана.
func FactDateFromModel(d *models.FactDate) *FactDate {
	if d == nil {
		return nil
	}

	return &FactDate{
		Year:      d.Year,
		Month:     d.Month,
		Day:       d.Day,
		Precision: string(d.Precision),
		Modifier:  string(d.Modifier),
		Calendar:  string(d.Calendar),
		YearTo:    d.YearTo,
		MonthTo:   d.MonthTo,
		DayTo:     d.DayTo,
	}
}

// Model конвертирует контракт обратно в модель; nil — дата не указана.
func (d *FactDate) Model() *models.FactDate {
	if d == nil {
		return nil
	}

	return &models.FactDate{
		Year:      d.Year,
		Month:     d.Month,
		Day:       d.Day,
		Precision: models.FactPrecision(d.Precision),
		Modifier:  models.FactModifier(d.Modifier),
		Calendar:  models.FactCalendar(d.Calendar),
		YearTo:    d.YearTo,
		MonthTo:   d.MonthTo,
		DayTo:     d.DayTo,
	}
}
