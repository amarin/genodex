package models

// RepositoryType — тип хранилища-контейнера источников (решение #25).
// Значения свободные и расширяемые.
type RepositoryType string

const (
	RepositoryTypeArchive RepositoryType = "archive"
	RepositoryTypeLibrary RepositoryType = "library"
	RepositoryTypeMuseum  RepositoryType = "museum"
	RepositoryTypePrivate RepositoryType = "private"
	RepositoryTypeOther   RepositoryType = "other"
)
