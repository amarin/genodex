package models

// AttachmentKind — тип файлового вложения.
type AttachmentKind string

const (
	AttachmentKindScan     AttachmentKind = "scan"
	AttachmentKindDocument AttachmentKind = "document"
	AttachmentKindAudio    AttachmentKind = "audio"
	AttachmentKindPhoto    AttachmentKind = "photo"
)
