package models

// Reliability — градация достоверности доказательства или утверждения.
type Reliability string

const (
	ReliabilityPrimary      Reliability = "primary"
	ReliabilityContemporary Reliability = "contemporary"
	ReliabilityMemory       Reliability = "memory"
	ReliabilityIndirect     Reliability = "indirect"
	ReliabilityUnknown      Reliability = "unknown"
)
