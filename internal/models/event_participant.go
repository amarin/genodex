package models

// EventParticipant — участник события с ролью в нём.
type EventParticipant struct {
	PersonID ID
	Role     string
	Note     string
}
