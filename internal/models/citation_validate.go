package models

// Validate проверяет цитату: источник обязателен и имеет тип source; якорь и
// текст необязательны (цитата может относиться к источнику целиком).
// Соответствие вида якоря Source.kind проверяют сценарии (нужно хранилище).
func (c *Citation) Validate() error {
	return finish(TypeCitation, c.validate())
}

func (c *Citation) validate() *ValidationError {
	if e := idErr("id", c.ID, TypeCitation); e != nil {
		return e
	}
	if e := idErr("source_id", c.SourceID, TypeSource); e != nil {
		return e
	}

	return validateAnchor(c.Anchor).within("anchor")
}
