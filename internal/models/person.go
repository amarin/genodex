package models

// PersonGender — пол персоны.
type PersonGender string

const (
	Male    PersonGender = "male"
	Female  PersonGender = "female"
	Unknown PersonGender = "unknown"
)

// Person — персона (внутренний тип сценариев).
// JSON-теги совпадают с internal/entity, чтобы сохранить внешние контракты.
type Person struct {
	ID         string            `json:"id,omitempty"`
	Surname    string            `json:"surname"`
	FirstName  string            `json:"first_name"`
	Patronymic string            `json:"patronymic,omitempty"`
	Gender     PersonGender      `json:"gender,omitempty"`
	Estates    []string          `json:"estates,omitempty"`
	Titles     []string          `json:"titles,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}
