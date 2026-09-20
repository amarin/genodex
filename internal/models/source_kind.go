package models

// SourceKind — тип доказательства.
type SourceKind string

const (
	SourceKindArchivalScan  SourceKind = "archival-scan"
	SourceKindTranscription SourceKind = "transcription"
	SourceKindDocument      SourceKind = "document"
	SourceKindAudio         SourceKind = "audio"
	SourceKindPhoto         SourceKind = "photo"
	SourceKindMemory        SourceKind = "memory"
	SourceKindExternal      SourceKind = "external"
)
