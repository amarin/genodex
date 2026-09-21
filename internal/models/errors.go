package models

import "errors"

// ErrNotFound — запрошенной сущности нет. Get-методы порта возвращают её
// (проверять через errors.Is); обработчики превращают в «не найдено».
var ErrNotFound = errors.New("не найдено")
