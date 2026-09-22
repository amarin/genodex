package auth

import "fmt"

// ValidationError — нарушение инварианта auth-сущности.
type ValidationError struct {
	Field  string
	Reason string
	Err    error
}

func (e *ValidationError) Unwrap() error { return e.Err }

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Reason
	}

	return e.Field + ": " + e.Reason
}

func fieldErr(field, format string, args ...any) *ValidationError {
	return &ValidationError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

func idFieldErr(field string, id ID, want Kind) *ValidationError {
	err := id.Validate(want)
	if err == nil {
		return nil
	}

	return &ValidationError{Field: field, Reason: err.Error(), Err: err}
}

// validatePassword проверяет сырой (нехешированный) пароль перед хешированием:
// непустой и не длиннее 72 байт — жёсткий лимит bcrypt на длину входа (это
// байты, не руны: нелатинские алфавиты вроде кириллицы занимают несколько
// байт на символ). Не применяется к уже сохранённому хешу или к сравниваемому
// текущему паролю (ChangePassword) — только к новому паролю перед хешем.
func validatePassword(field, password string) error {
	if password == "" {
		return fieldErr(field, "не может быть пустым")
	}
	if len(password) > 72 {
		return fieldErr(field, "не может быть длиннее 72 байт (ограничение bcrypt)")
	}

	return nil
}
