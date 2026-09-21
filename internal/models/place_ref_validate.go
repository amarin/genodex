package models

import "strings"

// placeTypes — типы сущностей, на которые может указывать PlaceRef.
var placeTypes = [...]Type{TypeAdministrativeDivision, TypeChurch, TypeParish}

// Validate проверяет указание на место: как TextRef, а ссылка (если есть) —
// только на административное деление, церковь или приход.
func (p PlaceRef) Validate() error {
	return finish("", p.validate())
}

func (p PlaceRef) validate() *ValidationError {
	r := TextRef(p)
	if e := r.validateAs(""); e != nil {
		return e
	}
	if r.Ref == "" {
		return nil
	}

	for _, t := range placeTypes {
		if r.Type == t {
			return nil
		}
	}

	names := make([]string, len(placeTypes))
	for i, t := range placeTypes {
		names[i] = string(t)
	}

	return fieldErr("type", "место может ссылаться только на %s, получено %s", strings.Join(names, ", "), r.Type)
}
