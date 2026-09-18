package entity

type EventType string

const (
	EventTypeBirth    EventType = "birth"
	EventTypeDeath    EventType = "death"
	EventTypeMarriage EventType = "marriage"
)
