package models

import (
	"errors"
	"testing"
)

// Новый тип сущности без валидатора не проходит: каждый Type из AllTypes
// обязан иметь сущность с Validate() и согласованным EntityType().
func TestEveryEntityTypeHasValidator(t *testing.T) {
	type validatable interface {
		Validate() error
		EntityType() Type
	}

	entities := map[Type]validatable{
		TypePerson:                 &Person{},
		TypeRelation:               &Relation{},
		TypeResidence:              &Residence{},
		TypeFamily:                 &Family{},
		TypeSurname:                &Surname{},
		TypeGivenName:              &GivenName{},
		TypePatronymic:             &Patronymic{},
		TypeEstate:                 &Estate{},
		TypeTitle:                  &Title{},
		TypeAdministrativeDivision: &AdministrativeDivision{},
		TypeChurch:                 &Church{},
		TypeParish:                 &Parish{},
		TypeEvent:                  &Event{},
		TypeSource:                 &Source{},
		TypeCitation:               &Citation{},
		TypeNote:                   &Note{},
		TypeRepository:             &Repository{},
		TypeArchive:                &Archive{},
		TypeArchiveNode:            &ArchiveNode{},
		TypeArchiveDocument:        &ArchiveDocument{},
		TypeAttachment:             &Attachment{},
	}

	if len(entities) != len(AllTypes()) {
		t.Errorf("сущностей в таблице %d, типов в AllTypes %d", len(entities), len(AllTypes()))
	}

	for _, typ := range AllTypes() {
		e, ok := entities[typ]
		if !ok {
			t.Errorf("для типа %s нет сущности с Validate()", typ)

			continue
		}
		if got := e.EntityType(); got != typ {
			t.Errorf("%s: EntityType() = %s", typ, got)
		}

		// Нулевое значение невалидно (id обязателен) и указывает свой тип.
		var ve *ValidationError
		if err := e.Validate(); !errors.As(err, &ve) {
			t.Errorf("%s: Validate() нулевого значения = %v, want *ValidationError", typ, err)
		} else if ve.Entity != typ {
			t.Errorf("%s: ValidationError.Entity = %s", typ, ve.Entity)
		}
	}
}
