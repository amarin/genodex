package models

// FactCalendar — календарь, в котором записана дата.
type FactCalendar string

const (
	FactCalendarGregorian FactCalendar = "gregorian"
	FactCalendarJulian    FactCalendar = "julian"
	FactCalendarUnknown   FactCalendar = "unknown"
)
