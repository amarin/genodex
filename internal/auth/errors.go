package auth

import "errors"

// ErrNotFound — auth-сущность не найдена в Store.
var ErrNotFound = errors.New("не найдено")

// ErrInvalidCredentials — неверный логин/пароль, либо невалидный/отозванный
// токен. Один сентинел на все случаи «кто-то не тот» — не различаем
// «нет такого логина» и «неверный пароль» (защита от перебора логинов).
var ErrInvalidCredentials = errors.New("неверный логин или пароль")

// ErrSessionExpired — refresh-токен просрочен или не найден.
var ErrSessionExpired = errors.New("сессия истекла")

// ErrInviteRequired — регистрация после bootstrap требует invite-ссылку.
var ErrInviteRequired = errors.New("нужна invite-ссылка")

// ErrInviteInvalid — invite-ссылка не найдена, использована или истекла.
var ErrInviteInvalid = errors.New("invite-ссылка недействительна")

// ErrLoginTaken — логин уже занят другим владельцем.
var ErrLoginTaken = errors.New("логин уже занят")
