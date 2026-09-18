package entity

type Patronymic string

// PatronymicSemantic задаёт связь отчества с именем родителя
type PatronymicSemantic struct {
	Patronymic Patronymic `json:"patronymic"`
	NameGender NameGender `json:"nameGender"`
	ParentName Name       `json:"parentName,omitempty"`
}
