package models

// EventType — тип события (открытый набор: значения расширяются без миграции).
type EventType string

const (
	EventTypeBirth      EventType = "birth"
	EventTypeDeath      EventType = "death"
	EventTypeMarriage   EventType = "marriage"
	EventTypeBurial     EventType = "burial"
	EventTypeConfession EventType = "confession"
	EventTypeCensus     EventType = "census"
)
