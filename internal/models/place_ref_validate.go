package models

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

	return fieldErr("type", "место может ссылаться только на administrative_division, church или parish, получено %s", r.Type)
}
