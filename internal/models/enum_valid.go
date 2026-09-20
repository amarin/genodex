package models

// Valid сообщает, что тип сущности известен (есть префикс идентификатора).
func (t Type) Valid() bool { return t.IDPrefix() != "" }

// Valid сообщает, что значение — одна из констант.
func (g PersonGender) Valid() bool {
	switch g {
	case Male, Female, Unknown:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (g NameGender) Valid() bool {
	switch g {
	case MaleName, FemaleName, NeutralName:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (t PersonNameType) Valid() bool {
	switch t {
	case PersonNameMain, PersonNameBirth, PersonNameMarried, PersonNameChanged, PersonNamePseudonym:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (k RelationKind) Valid() bool {
	switch k {
	case RelationKindBlood, RelationKindMarriage, RelationKindAdoption, RelationKindAssociate:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (k SourceKind) Valid() bool {
	switch k {
	case SourceKindArchivalScan, SourceKindTranscription, SourceKindDocument,
		SourceKindAudio, SourceKindPhoto, SourceKindMemory, SourceKindExternal:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (k AttachmentKind) Valid() bool {
	switch k {
	case AttachmentKindScan, AttachmentKindDocument, AttachmentKindAudio, AttachmentKindPhoto:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (r Reliability) Valid() bool {
	switch r {
	case ReliabilityPrimary, ReliabilityContemporary, ReliabilityMemory,
		ReliabilityIndirect, ReliabilityUnknown:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (k AnchorKind) Valid() bool {
	switch k {
	case AnchorArchive, AnchorFile, AnchorURL:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (p FactPrecision) Valid() bool {
	switch p {
	case PrecisionUnknown, PrecisionYear, PrecisionMonth, PrecisionDay:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (m FactModifier) Valid() bool {
	switch m {
	case ModifierExact, ModifierApprox, ModifierBefore, ModifierAfter, ModifierBetween:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (c FactCalendar) Valid() bool {
	switch c {
	case FactCalendarGregorian, FactCalendarJulian, FactCalendarUnknown:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант (единицы деления и виды
// населённых пунктов).
func (t AdminDivisionType) Valid() bool {
	switch t {
	case AdminDivisionGovernorate, AdminDivisionDistrict, AdminDivisionVolost, AdminDivisionOther:
		return true
	}

	return t.IsSettlement()
}
