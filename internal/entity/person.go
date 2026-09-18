package entity

// Person сущность персоны
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

// Type возвращает тип сущности
func (p *Person) Type() EntityType {
	return TypePerson
}
